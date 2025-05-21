package funnel

import (
	"fmt"
	"html/template"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"strconv"

	"io/fs"

	"github.com/jonson/tsgrok/internal/util"
	"github.com/jonson/tsgrok/web"
)

var HttpServerPath = fmt.Sprintf("/%s/", util.ProgramName)

type HttpServer struct {
	port                  int             // port we're listening on
	mux                   *http.ServeMux  // mux for handling requests
	requestLimitPerFunnel int             // max requests per funnel to keep, older ones will be dropped
	messageBus            util.MessageBus // message bus for sending messages to the program
	funnelRegistry        *FunnelRegistry // registry of funnels
	logger                *stdlog.Logger  // logger for logging
	embeddedTemplates     *template.Template
}

func NewHttpServer(port int, messageBus util.MessageBus, funnelRegistry *FunnelRegistry, logger *stdlog.Logger) (*HttpServer, error) {
	// Parse templates from the web.TemplatesFS, first inject a few functions
	tmpl, err := loadTemplates()
	if err != nil {
		return nil, err
	}

	return &HttpServer{
		port:                  port,
		mux:                   http.NewServeMux(),
		requestLimitPerFunnel: 100,
		messageBus:            messageBus,
		funnelRegistry:        funnelRegistry,
		logger:                logger,
		embeddedTemplates:     tmpl,
	}, nil
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

	staticFilesRoot, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		s.logger.Fatalf("FATAL: 'static' subdirectory not found in embedded StaticFS: %v", err)
		return err
	}
	fileServer := http.FileServer(http.FS(staticFilesRoot))
	s.mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	s.mux.HandleFunc(HttpServerPath, s.handleRequest)
	s.mux.HandleFunc("/inspect/", s.handleFunnelInspect)
	s.mux.HandleFunc("/", s.handleRoot)

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
