package domain

import (
	trip "ride-sharing/services/trip-service/pkg/types"
	pb "ride-sharing/shared/proto/trip"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RideFareModel struct {
	ID              primitive.ObjectID
	UserId          string
	PackageSlug     string // ex: van, luxury, sedan
	TotalPriceCents float64
	ExpiresAt       time.Time
	Route           *trip.OsrmApiResponse
}

func (r *RideFareModel) ToProto() *pb.RideFare {
	return &pb.RideFare{
		Id:                r.ID.Hex(),
		UserID:            r.UserId,
		PackageSlug:       r.PackageSlug,
		TotalPriceInCents: r.TotalPriceCents,
	}
}

func ToRideFaresProto(fares []*RideFareModel) []*pb.RideFare {
	var protoFares []*pb.RideFare
	for _, q := range fares {
		protoFares = append(protoFares, q.ToProto())
	}
	return protoFares
}
