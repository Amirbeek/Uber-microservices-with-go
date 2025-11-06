package domain

import (
	"context"
	trip "ride-sharing/services/trip-service/pkg/types"
	pbd "ride-sharing/shared/proto/driver"
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

func (t *TripModel) ToProto() pb.Trip {
	return pb.Trip{
		Id:           t.ID.Hex(),
		SelectedFare: func() *pb.RideFare { if t.RideFare != nil { return t.RideFare.ToProto() }; return nil }(),
		Route:        func() *pb.Route { if t.RideFare != nil && t.RideFare.Route != nil { return t.RideFare.Route.ToProto() }; return nil }(),
		Status:       t.Status,
		UserID:       t.UserId,
		Driver:       t.Driver,
	}
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
	SaveRideFare(ctx context.Context, f *RideFareModel) error
	GetRideFareByID(ctx context.Context, id string) (*RideFareModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, id string, status string, driver *pbd.Driver) error
}

type TripService interface {
	CreateTrip(ctx context.Context, fare *RideFareModel) (*TripModel, error)
	GetRoute(ctx context.Context, pickup, destination *types.Coordinate) (*trip.OsrmApiResponse, error)
	EstimatePackagesPriceWithRoute(route *trip.OsrmApiResponse) []*RideFareModel
	GenerateTripFares(
		ctx context.Context,
		fares []*RideFareModel,
		route *trip.OsrmApiResponse,
		userID string,
	) ([]*RideFareModel, error)

	GetAndValidateFare(ctx context.Context, fareID, userID string) (*RideFareModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error
}
