package main

import (
	"context"
	pb "ride-sharing/shared/proto/driver"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcHandler struct {
	pb.UnimplementedDriverServiceServer
	Service *Service
}

func NewGrpcHandler(s *grpc.Server, service *Service) {
	pb.RegisterDriverServiceServer(s, &grpcHandler{
		Service: service,
	})
}

func (h *grpcHandler) RegisterDriver(ctx context.Context, req *pb.RegisterDriverRequest) (*pb.RegisterDriverResponse, error) {
	driverId := req.GetDriverID()
	packageSlug := req.GetPackageSlug()

	d, err := h.Service.RegisterDriver(driverId, packageSlug)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to register driver: %v", err)
	}
	return &pb.RegisterDriverResponse{
		Driver: d,
	}, nil
}

func (h *grpcHandler) UnregisterDriver(ctx context.Context, req *pb.RegisterDriverRequest) (*pb.RegisterDriverResponse, error) {
	driverId := req.GetDriverID()
	h.Service.UnregisterDriver(driverId)
	return &pb.RegisterDriverResponse{
		Driver: &pb.Driver{
			Id: driverId,
		},
	}, nil
}
