// Package app is responsible for initializing all the services required to run this service
package app

import (
	"context"

	"github.com/emma-sleep/go-telemetry/mlog"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/niuroulamian/restful-api-service/internal/api"
	"github.com/niuroulamian/restful-api-service/internal/server"
	"github.com/niuroulamian/restful-api-service/internal/server/marshal"
)

// Config contains configuration of the application
type Config struct {
	Version string
}

// App represents the running application
type App struct {
	logger       *mlog.MLog
	cfg          Config
	port         string
	httpEndPoint string

	apiSrv     *api.Service
	grpcServer *grpc.Server
	proxy      *server.Service
	mux        *runtime.ServeMux

	done chan struct{}
}

// New returns an instance of the App with the given configuration
func New(cfg Config, logger *mlog.MLog) *App {
	return &App{
		cfg:    cfg,
		port:   ":8081",
		logger: logger,
	}
}

// Start starts all the services required to run application
func (a *App) Start(ctx context.Context) {
	a.done = make(chan struct{})
	if err := a.startServices(ctx); err != nil {
		a.logger.Error(ctx, "couldn't start the app")
		a.Stop()
		return
	}
	<-ctx.Done()
	a.Stop()
}

func (a *App) startServices(ctx context.Context) error {
	a.grpcServer = grpc.NewServer()
	// creating mux for gRPC gateway. This will multiplex or route request different gRPC service
	a.mux = runtime.NewServeMux(
		runtime.WithMarshalerOption(
			runtime.MIMEWildcard,
			&runtime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					EmitDefaultValues: true,
				},
			}),
		runtime.WithOutgoingHeaderMatcher(func(s string) (string, bool) {
			return s, true
		}),
		runtime.WithMarshalerOption("application/x-www-form-urlencoded", &marshal.Form{
			PB: &runtime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					UseProtoNames:     true,
					EmitDefaultValues: true,
				},
			},
		}),
	)
	a.httpEndPoint = "localhost" + a.port

	a.apiSrv = api.NewService(a.grpcServer, a.mux, a.httpEndPoint, a.logger)
	if err := a.apiSrv.Start(ctx); err != nil {
		return err
	}

	a.proxy = server.New(a.grpcServer, a.mux, a.port)
	if err := a.proxy.Start(ctx); err != nil {
		return err
	}
	return nil
}

// Stop stops the application
func (a *App) Stop() {
	a.grpcServer.GracefulStop()
	close(a.done)
}

// Done returns channel that is closed once the applicaiton exits
func (a *App) Done() <-chan struct{} {
	return a.done
}
