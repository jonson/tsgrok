package funnel

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *HttpServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	pathAfterPrefix := strings.TrimPrefix(r.URL.Path, HttpServerPath)

	funnelIdAndRest, err := extractFunnelIdAndRest(pathAfterPrefix)
	if err != nil {
		if err == ErrInvalidFunnelPath { // Direct comparison for package-level error vars
			http.Error(w, ErrInvalidFunnelPath.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

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
		if err == ErrFunnelNotFound { // Direct comparison
			http.Error(w, ErrFunnelNotFound.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError) // Catch-all for other errors from GetFunnelById
		}
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
	proxy.ErrorLog = s.logger

	originalDirector := proxy.Director

	requestResponse := CaptureRequestResponse{
		ID:        uuid.New().String(),
		FunnelID:  funnel.HTTPFunnel.id,
		Timestamp: time.Now(),
	}

	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		var reqBodyBytes []byte
		var err error
		if req.Body != nil && req.Body != http.NoBody {
			reqBodyBytes, err = io.ReadAll(req.Body)
			if err != nil {
				s.logger.Printf("Error reading request body: %v", err)
			} else {
				err = req.Body.Close()
				if err != nil {
					s.logger.Printf("Error closing request body: %v", err)
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
					s.logger.Printf("Error closing response body: %v", err)
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

	proxy.ServeHTTP(w, r)

	funnel.Requests.Add(requestResponse)
	s.messageBus.Send(ProxyRequestMsg{FunnelId: funnel.HTTPFunnel.id})
}
