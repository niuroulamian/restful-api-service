// Package server starts an HTTP server. It redirects requests to actual service that should process the requests.
package server

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Service represents proxy service
type Service struct {
	port    string
	gs      *grpc.Server
	mux     *runtime.ServeMux
	statics map[string][]byte
	FS      fs.FS

	done chan struct{}
}

// Done returns a channel that is closed once the service exited
func (s *Service) Done() <-chan struct{} {
	return s.done
}

// Stop stops the service
func (s *Service) Stop() {
	close(s.done)
}

// New returns an instance of the service with the given configuration
func New(grpcServer *grpc.Server, mux *runtime.ServeMux, port string) *Service {
	return &Service{
		port:    port,
		gs:      grpcServer,
		mux:     mux,
		statics: make(map[string][]byte),
	}
}

// Start starts service
func (s *Service) Start(ctx context.Context) error {
	s.done = make(chan struct{})
	err := s.loadStatics(ctx)
	if err != nil {
		return err
	}
	// creating a listener for server
	l, err := net.Listen("tcp", s.port)
	if err != nil {
		return err
	}
	m := cmux.New(l)
	// a different listener for HTTP1
	httpL := m.Match(cmux.HTTP1Fast())
	// a different listener for HTTP2 since gRPC uses HTTP2
	grpcL := m.Match(cmux.HTTP2())

	// passing dummy listener
	go func() {
		_ = s.gs.Serve(grpcL)
	}()

	// Creating a normal HTTP server
	server := http.Server{
		ReadHeaderTimeout: 60 * time.Second,
		Handler:           withLogger(serveHTTP(s.mux, s.statics, s.FS)),
	}
	// start server
	// passing dummy listener
	go func() {
		_ = server.Serve(httpL)
	}()

	// actual listener
	go func() {
		_ = m.Serve()
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return nil
}

func withLogger(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		m := httpsnoop.CaptureMetrics(handler, writer, request)
		log.Printf("http[%d]-- %s -- %s\n", m.Code, m.Duration, request.URL.Path)
	})
}

//go:embed statics
var staticsFS embed.FS

func (s *Service) loadStatics(ctx context.Context) error {
	files, err := fs.Sub(staticsFS, "statics")
	if err != nil {
		return err
	}
	s.FS = files
	err = fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		s.statics[path], err = fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

func serveHTTP(handler http.Handler, statics map[string][]byte, FS fs.FS) http.Handler {
	sm := http.NewServeMux()
	sm.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		data, ok := statics["index.html"]
		if !ok {
			zap.S().Error("index.html not found")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	})
	sm.Handle("/", http.FileServer(http.FS(FS)))
	sm.Handle("/api/", handler)
	return sm
}
