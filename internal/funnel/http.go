package funnel

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonson/tsgrok/internal/util"
)

// Error variables for common failure modes
var (
	ErrInvalidFunnelPath = errors.New("invalid path format for funnel request")
	ErrFunnelNotFound    = errors.New("funnel not found")
	ErrFunnelNotReady    = errors.New("funnel has no local target configured")
	ErrTargetURLParse    = errors.New("failed to parse funnel target URL")
)

var HttpServerPath = fmt.Sprintf("/%s/", util.ProgramName)

type HttpServer struct {
	port                  int             // port we're listening on
	mux                   *http.ServeMux  // mux for handling requests
	requestLimitPerFunnel int             // max requests per funnel to keep, older ones will be dropped
	messageBus            util.MessageBus // message bus for sending messages to the program
	funnelRegistry        *FunnelRegistry // registry of funnels
	logger                *stdlog.Logger  // logger for logging
}

func NewHttpServer(port int, messageBus util.MessageBus, funnelRegistry *FunnelRegistry, logger *stdlog.Logger) *HttpServer {
	return &HttpServer{
		port:                  port,
		mux:                   http.NewServeMux(),
		requestLimitPerFunnel: 100,
		messageBus:            messageBus,
		funnelRegistry:        funnelRegistry,
		logger:                logger,
	}
}

func (s *HttpServer) GetFunnelById(id string) (Funnel, error) {
	return s.funnelRegistry.GetFunnel(id)
}

func (s *HttpServer) Start() error {

	target := "localhost:" + strconv.Itoa(s.port)

	// quick check if the port is available, fail fast if we can't bind
	listener, err := net.Listen("tcp", target)
	if err != nil {
		return err
	}
	err = listener.Close()
	if err != nil {
		return err
	}

	s.mux.HandleFunc(HttpServerPath, s.handleRequest)

	// do this in a goroutine, we listen in the background
	go func() {
		server := &http.Server{Addr: target, Handler: s.mux, ErrorLog: s.logger}
		err := server.ListenAndServe()

		if err != nil {
			// this will kill the program
			s.logger.Println(err)
			os.Exit(1)
		}
	}()

	return nil
}

func (s *HttpServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	pathAfterPrefix := strings.TrimPrefix(r.URL.Path, HttpServerPath)

	funnelIdAndRest, err := extractFunnelIdAndRest(pathAfterPrefix)
	if err != nil {
		// Check for the specific error from extraction
		if errors.Is(err, ErrInvalidFunnelPath) {
			http.Error(w, ErrInvalidFunnelPath.Error(), http.StatusBadRequest)
		} else {
			// Handle other unexpected errors during extraction
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// serve hello requests without proxying
	if funnelIdAndRest.rest == ".well-known/tsgrok/hello" {
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte("hello"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	funnel, err := s.GetFunnelById(funnelIdAndRest.id)
	if err != nil {
		http.Error(w, ErrFunnelNotFound.Error(), http.StatusNotFound)
		return
	}

	targetURLStr := funnel.LocalTarget()
	if targetURLStr == "" {
		http.Error(w, ErrFunnelNotReady.Error(), http.StatusNotFound)
		return
	}

	targetURL, err := url.Parse(targetURLStr)
	if err != nil {
		s.logger.Printf("Error parsing target URL %q: %v", targetURLStr, err)
		http.Error(w, ErrTargetURLParse.Error(), http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// need this to avoid logging to stderr
	proxy.ErrorLog = s.logger

	// Define a custom Director
	originalDirector := proxy.Director

	requestResponse := CaptureRequestResponse{
		ID:        uuid.New().String(),
		FunnelID:  funnel.HTTPFunnel.id,
		Timestamp: time.Now(),
	}

	// this is the function that modifies the request before it is sent to the target
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// read the request body, the plan is to expose this in the UI somehow, but that comes 
		// at the expense of increased memory usage...  make this better
		var reqBodyBytes []byte
		var err error
		if req.Body != nil && req.Body != http.NoBody {
			reqBodyBytes, err = io.ReadAll(req.Body)
			if err != nil {
				s.logger.Printf("Error reading request body: %v\n", err)
			} else {
				err = req.Body.Close()
				if err != nil {
					s.logger.Printf("Error closing request body: %v\n", err)
				}
				req.Body = io.NopCloser(bytes.NewReader(reqBodyBytes))
				req.ContentLength = int64(len(reqBodyBytes))
				req.GetBody = nil
			}
		}

		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.URL.Path = singleJoiningSlash(targetURL.Path, funnelIdAndRest.rest)
		req.Host = targetURL.Host

		if targetURL.RawPath == "" {
			req.URL.RawPath = ""
		}

		headers := make(map[string]string)
		for k, v := range req.Header {
			headers[k] = strings.Join(v, ",")
		}

		requestResponse.Request = CaptureRequest{
			Method:  req.Method,
			URL:     req.URL.String(),
			Body:    reqBodyBytes,
			Headers: headers,
		}
	}

	// this is the function that modifies the response before it is sent to the client
	proxy.ModifyResponse = func(resp *http.Response) error {

		headers := make(map[string]string)
		for k, v := range resp.Header {
			headers[k] = strings.Join(v, ",")
		}

		requestResponse.Response = CaptureResponse{
			Headers:    headers,
			StatusCode: resp.StatusCode,
		}

		var respBodyBytes []byte
		var err error
		if resp.Body != nil && resp.Body != http.NoBody {
			respBodyBytes, err = io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			} else {
				err = resp.Body.Close()
				if err != nil {
					s.logger.Printf("Error closing response body: %v\n", err)
				}
				resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
				resp.ContentLength = int64(len(respBodyBytes))
				resp.Header.Del("Transfer-Encoding")
			}
		}

		requestResponse.Response.Body = respBodyBytes
		requestResponse.Duration = time.Since(requestResponse.Timestamp)
		return nil
	}

	// Serve the request via the proxy
	proxy.ServeHTTP(w, r)

	// add the request response to the list
	funnel.Requests.Add(requestResponse)

	// broadcast it so UI can update
	s.messageBus.Send(ProxyRequestMsg{FunnelId: funnel.HTTPFunnel.id})
}

type FunnelIdAndRest struct {
	id   string
	rest string
}

func extractFunnelIdAndRest(pathAfterPrefix string) (FunnelIdAndRest, error) {
	// Check for obviously invalid paths first
	if pathAfterPrefix == "" || pathAfterPrefix == "/" {
		return FunnelIdAndRest{}, ErrInvalidFunnelPath
	}

	// Split the remaining path by /
	parts := strings.SplitN(pathAfterPrefix, "/", 2)

	funnelId := ""
	rest := ""

	if len(parts) >= 1 {
		funnelId = parts[0]
	}
	if len(parts) == 2 {
		rest = parts[1]
	}

	// Check if funnelId is empty after splitting (e.g., path started with /)
	if funnelId == "" {
		return FunnelIdAndRest{}, ErrInvalidFunnelPath // Use the specific error
	}

	return FunnelIdAndRest{id: funnelId, rest: rest}, nil
}

func singleJoiningSlash(a, b string) string {
	if a == "" && b == "" {
		return "/"
	}
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		// Avoid adding slash if b is empty or a is just "/"
		if b == "" || a == "/" {
			return a + b
		}
		return a + "/" + b
	}
	return a + b
}
