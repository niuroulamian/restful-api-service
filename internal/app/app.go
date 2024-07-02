// Package app is responsible for initializing all the services required to run this service
package app

import (
	"context"

	"go.mxc.org/external-api/internal/mxpsrv"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"

	"go.uber.org/zap"

	"go.mxc.org/external-api/internal/client/mxpsrvcli"
	"go.mxc.org/external-api/internal/client/neosrvcli"
	"go.mxc.org/external-api/internal/grpcauth"
	"go.mxc.org/external-api/internal/mockcustody"
	"go.mxc.org/external-api/internal/neosrv"
	"go.mxc.org/external-api/internal/server"
	"go.mxc.org/external-api/internal/server/marshal"
	"go.mxc.org/grpcutil/grpccon"
	"go.mxc.org/mlog"
	"go.mxc.org/monitor"
	"go.mxc.org/prompublisher"
	"go.mxc.org/usersrv/pkg/authcli"
)

// Config contains configuration of the application
type Config struct {
	Logging mlog.Config `mapstructure:"logging"`

	Version string
}

// App represents the running application
type App struct {
	logger       *zap.SugaredLogger
	cfg          Config
	port         string
	httpEndPoint string
	mon          *monitor.Monitor
	grpcServer   *grpc.Server
	mux          *runtime.ServeMux
	neosrvCli    *neosrvcli.NEOServiceClient
	mxpsrvCli    *mxpsrvcli.MXPServiceClient
	// authentication client
	authCli *authcli.AuthCli

	mockCustody *mockcustody.Service
	proxy       *server.Service
	neosrv      *neosrv.Service
	mxpsrv      *mxpsrv.Service
}

// New returns an instance of the App with the given configuration
func New(cfg Config) *App {
	return &App{
		cfg:  cfg,
		port: ":8081",
	}
}

// Start starts all the services required to run application
func (a *App) Start(ctx context.Context) {
	a.logger = mlog.Extract(ctx)
	a.mon, ctx = monitor.New(ctx)
	go func() {
		if err := a.startServices(ctx); err != nil {
			mlog.Extract(ctx).With(zap.Error(err)).Error("couldn't start the app")
			a.Stop()
			return
		}
		mlog.Extract(ctx).Debug("app is running")
		<-ctx.Done()
		a.Stop()
	}()
}

func (a *App) startServices(ctx context.Context) error {
	err := a.establishExternalConnections(ctx)
	if err != nil {
		return err
	}
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

	err = a.startAPIService(ctx)
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

func (a *App) establishExternalConnections(ctx context.Context) error {
	var err error
	a.neosrvCli, err = neosrvcli.Connect(ctx, a.cfg.NeoServiceClient)
	if err != nil {
		return err
	}
	a.authCli, err = authcli.New(a.cfg.UserSrvClient)
	if err != nil {
		return err
	}
	a.mxpsrvCli, err = mxpsrvcli.Connect(ctx, a.cfg.MXPServiceClient)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) closeExternalConnections() {
	err := a.neosrvCli.Close()
	if err != nil {
		a.logger.Error("couldn't close connection to neo service server")
	}
	err = a.authCli.Close()
	if err != nil {
		a.logger.Error("couldn't close connection to auth server")
	}
	err = a.mxpsrvCli.Close()
	if err != nil {
		a.logger.Error("couldn't close connection to mxp server")
	}
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
	a.closeExternalConnections()
	a.mon.Cancel()
	a.grpcServer.GracefulStop()
}

// Done returns channel that is closed once the applicaiton exits
func (a *App) Done() <-chan struct{} {
	return a.mon.Done()
}
