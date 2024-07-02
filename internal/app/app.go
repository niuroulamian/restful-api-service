// Package app is responsible for initializing all the services required to run this service
package app

import (
	"context"
	"github.com/emma-sleep/go-telemetry/mlog"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/niuroulamian/restful-api-service/internal/server"
	"github.com/niuroulamian/restful-api-service/internal/server/marshal"
)

// Config contains configuration of the application
type Config struct {
	Logging zap.Config `mapstructure:"logging"`

	Version string
}

// App represents the running application
type App struct {
	logger       *mlog.MLog
	cfg          Config
	port         string
	httpEndPoint string
	grpcServer   *grpc.Server
	proxy        *server.Service

	mux  *runtime.ServeMux
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
				EnumsAsInts:  false,
				EmitDefaults: true,
			}),
		runtime.WithOutgoingHeaderMatcher(func(s string) (string, bool) {
			return s, true
		}),
		runtime.WithMarshalerOption("application/x-www-form-urlencoded", &marshal.Form{
			PB: &runtime.JSONPb{
				OrigName:     true,
				EmitDefaults: true,
			},
		}),
	)
	a.httpEndPoint = "localhost" + a.port

	err := a.startAPIService(ctx)
	if err != nil {
		return err
	}

	a.proxy = server.New(a.grpcServer, a.mux, a.port)
	if err := a.proxy.Start(ctx); err != nil {
		return err
	}
	a.mon.Watch(ctx, "proxy", a.proxy)
	return nil
}

func (a *App) startAPIService(ctx context.Context) (err error) {
	ga := grpcauth.New(a.authCli)

	a.mockCustody = mockcustody.New(ctx, a.grpcServer, a.mux, a.httpEndPoint)
	err = a.mockCustody.Start(ctx, a.cfg.MockCustodyClient)
	if err != nil {
		return err
	}
	a.mon.Watch(ctx, "mock custody", a.mockCustody)

	a.neosrv = neosrv.New(ctx, a.grpcServer, a.mux, a.httpEndPoint, a.neosrvCli.MinerSrvCli, a.neosrvCli.BondSrvCli)
	err = a.neosrv.Start(ctx, ga)
	if err != nil {
		return err
	}
	a.mon.Watch(ctx, "neo service", a.neosrv)

	a.mxpsrv = mxpsrv.New(ctx, a.grpcServer, a.mux, a.httpEndPoint,
		a.mxpsrvCli.WalletSrvCli, a.neosrvCli.BondSrvCli, a.mxpsrvCli.WithdrawSrvCli)
	err = a.mxpsrv.Start(ctx, ga)
	if err != nil {
		return err
	}
	a.mon.Watch(ctx, "mxp service", a.mxpsrv)

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
