package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RideFareModel struct {
	ID              primitive.ObjectID
	UserId          string
	PackageSlug     string // ex: van, luxury, sedan
	TotalPriceCents float64
	ExpiresAt       time.Time
}
