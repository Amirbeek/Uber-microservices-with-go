package domain

import (
	"context"
	trip "ride-sharing/services/trip-service/pkg/types"
	pb "ride-sharing/shared/proto/trip"
	"ride-sharing/shared/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TripModel struct {
	ID       primitive.ObjectID
	UserId   string
	Status   string
	RideFare *RideFareModel
	Driver   *pb.TripDriver
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
	SaveRideFare(ctx context.Context, f *RideFareModel) error
	GetRideFareByID(ctx context.Context, id string) (*RideFareModel, error)
}

type TripService interface {
	CreateTrip(ctx context.Context, fare *RideFareModel) (*TripModel, error)
	GetRoute(ctx context.Context, pickup, destination *types.Coordinate) (*trip.OsrmApiResponse, error)
	EstimatePackagesPriceWithRoute(route *trip.OsrmApiResponse) []*RideFareModel
	GenerateTripFares(
		ctx context.Context,
		fares []*RideFareModel,
		route *trip.OsrmApiResponse,
		userID string) ([]*RideFareModel, error)

	GetAndValidateFare(ctx context.Context, fareID, userID string) (*RideFareModel, error)
}
