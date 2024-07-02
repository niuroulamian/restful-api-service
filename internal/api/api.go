package api

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pb "github.com/niuroulamian/restful-api-service/go/v1"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/emma-sleep/go-telemetry/mlog"
	"google.golang.org/grpc"
)

type Service struct {
	pb.UnimplementedMockAPIServiceServer

	endpoint string
	gs       *grpc.Server
	mux      *runtime.ServeMux
	logger   *mlog.MLog

	done chan struct{}
}

// GetServiceInfo returns a response
func (s *Service) GetServiceInfo(ctx context.Context, req *pb.GetServiceInfoRequest) (*pb.GetServiceInfoResponse, error) {
	return &pb.GetServiceInfoResponse{
		Response: fmt.Sprintf("Hello, %s", req.ServiceId),
	}, nil
}

func NewService(gs *grpc.Server, mux *runtime.ServeMux, endpoint string, logger *mlog.MLog) *Service {
	return &Service{
		gs:       gs,
		logger:   logger,
		mux:      mux,
		endpoint: endpoint,
	}
}

// Stop stops the service
func (s *Service) Stop() {
	close(s.done)
}

// Done returns a channel that is closed once the service exited
func (s *Service) Done() <-chan struct{} {
	return s.done
}

func (s *Service) Start(ctx context.Context) error {
	s.done = make(chan struct{})

	pb.RegisterMockAPIServiceServer(s.gs, s)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	err := pb.RegisterMockAPIServiceHandlerFromEndpoint(ctx, s.mux, s.endpoint, opts)
	if err != nil {
		s.logger.Error(ctx, "couldn't register the service")
		s.Stop()
		return err
	}
	return nil
}
