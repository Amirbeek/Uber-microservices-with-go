package grpc

import (
	"context"
	"log"
	"ride-sharing/services/trip-service/internal/domain"
	"ride-sharing/services/trip-service/internal/infrastructure/events"
	pb "ride-sharing/shared/proto/trip"
	"ride-sharing/shared/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gRPCHandler struct {
	pb.UnimplementedTripServiceServer
	service   domain.TripService
	publisher *events.TripEventPublisher
}

func NewGRPCHandler(server *grpc.Server, service domain.TripService, publisher *events.TripEventPublisher) *gRPCHandler {
	handler := &gRPCHandler{
		service:   service,
		publisher: publisher,
	}
	pb.RegisterTripServiceServer(server, handler)
	return handler
}

func (h *gRPCHandler) CreateTrip(ctx context.Context, req *pb.CreateTripRequest) (*pb.CreateTripResponse, error) {
	fareID := req.GetRideFareID()
	userID := req.GetUserID()

	//	1. Fetch and validate the fare.
	ride, err := h.service.GetAndValidateFare(ctx, fareID, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to validate the fare: %v", err)
	}
	// 2. Call create trip.
	trip, err := h.service.CreateTrip(ctx, ride)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create the trip: %v", err)
	}
	// 3. We also need to initialize an empty driver to the trip

	if err := h.publisher.PublishTripCreated(ctx, trip); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to publish the trip: %v", err)
	}
	// 4. Add a comment at the end of the function to publish
	return &pb.CreateTripResponse{
		TripID: trip.ID.Hex(),
	}, nil
}

func (h *gRPCHandler) PreviewTrip(ctx context.Context, req *pb.PreviewTripRequest) (*pb.PreviewTripResponse, error) {
	pickup := req.GetStartLocation()
	destination := req.GetEndLocation()

	pickupCoord := &types.Coordinate{
		Latitude:  pickup.Latitude,
		Longitude: pickup.Longitude,
	}
	destinationCoord := &types.Coordinate{
		Latitude:  destination.Latitude,
		Longitude: destination.Longitude,
	}

	userID := req.GetUserID()
	route, err := h.service.GetRoute(ctx, pickupCoord, destinationCoord)
	if err != nil {
		log.Printf("failed to get route: %v", err)
		return nil, status.Error(codes.Internal, "failed to get route")
	}
	estimatedFares := h.service.EstimatePackagesPriceWithRoute(route)

	fares, err := h.service.GenerateTripFares(ctx, estimatedFares, route, userID)

	if err != nil {
		log.Printf("failed to generate trip: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate trip")
	}
	return &pb.PreviewTripResponse{
		Route:     route.ToProto(),
		RideFares: domain.ToRideFaresProto(fares),
	}, nil
}
