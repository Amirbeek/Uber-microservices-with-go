package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ride-sharing/services/trip-service/internal/domain"
	trip "ride-sharing/services/trip-service/pkg/types"
	pbd "ride-sharing/shared/proto/driver"
	tripshared "ride-sharing/shared/proto/trip"
	"ride-sharing/shared/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type service struct {
	repo domain.TripRepository
}

func NewTripService(repo domain.TripRepository) *service {
	return &service{repo: repo}
}

func (s *service) CreateTrip(ctx context.Context, fare *domain.RideFareModel) (*domain.TripModel, error) {
	tr := &domain.TripModel{
		ID:       primitive.NewObjectID(),
		UserId:   fare.UserId,
		Status:   "pending",
		RideFare: fare,
		Driver:   &tripshared.TripDriver{},
	}
	t, err := s.repo.CreateTrip(ctx, tr)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *service) GetAndValidateFare(ctx context.Context, fareID, userID string) (*domain.RideFareModel, error) {
	fare, err := s.repo.GetRideFareByID(ctx, fareID)
	if err != nil {
		return nil, err
	}
	// Validation user Fare
	if fare == nil {
		return nil, fmt.Errorf("fare not found")
	}
	if userID != fare.UserId {
		return nil, fmt.Errorf("fare does not belong to the user")
	}

	return fare, nil
}

func (s *service) GetRoute(ctx context.Context, pickup, destination *types.Coordinate) (*trip.OsrmApiResponse, error) {
	url := fmt.Sprintf("http://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson",
		pickup.Longitude, pickup.Latitude, destination.Longitude, destination.Latitude)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch route from OSRM API: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch route from response %s", err)
	}
	var routeResp trip.OsrmApiResponse
	if err := json.Unmarshal(body, &routeResp); err != nil {
		return nil, fmt.Errorf("failed parse json: %s", err)
	}

	return &routeResp, nil
}

func (s *service) EstimatePackagesPriceWithRoute(route *trip.OsrmApiResponse) []*domain.RideFareModel {
	baseFares := getBaseFares()

	estimatedFares := make([]*domain.RideFareModel, len(baseFares))
	for i, fare := range baseFares {
		estimatedFares[i] = s.estimateFareRoute(fare, route)
	}
	return estimatedFares
}

func (s *service) GenerateTripFares(ctx context.Context, f []*domain.RideFareModel, route *trip.OsrmApiResponse, userID string) ([]*domain.RideFareModel, error) {
	fares := make([]*domain.RideFareModel, len(f))

	for i, fare := range f {
		id := primitive.NewObjectID()
		fare := &domain.RideFareModel{
			UserId:          userID,
			ID:              id,
			TotalPriceCents: fare.TotalPriceCents,
			PackageSlug:     fare.PackageSlug,
			Route:           route,
		}
		if err := s.repo.SaveRideFare(ctx, fare); err != nil {
			return nil, fmt.Errorf("failed to save ride fare: %s", err)
		}
		fares[i] = fare
	}

	return fares, nil
}

func (s *service) estimateFareRoute(f *domain.RideFareModel, route *trip.OsrmApiResponse) *domain.RideFareModel {
	// distance
	// time

	pricingConfing := trip.DefaultPricingConfig()
	CarPackagePrice := f.TotalPriceCents

	distanceKm := route.Routes[0].Distance
	durationInMinutes := route.Routes[0].Duration

	// distance
	distanceFare := distanceKm * pricingConfing.PricePerDistance

	// time
	timeFare := durationInMinutes * pricingConfing.PricingPerMinute

	// car price
	totalPrice := CarPackagePrice*timeFare + distanceFare

	return &domain.RideFareModel{
		TotalPriceCents: totalPrice,
		PackageSlug:     f.PackageSlug,
	}

}

func getBaseFares() []*domain.RideFareModel {
	return []*domain.RideFareModel{
		{
			PackageSlug:     "sedan",
			TotalPriceCents: 450,
		},
		{
			PackageSlug:     "suv",
			TotalPriceCents: 200,
		},
		{
			PackageSlug:     "van",
			TotalPriceCents: 400,
		},
		{
			PackageSlug:     "luxury",
			TotalPriceCents: 1000,
		},
	}
}

func (s *service) GetTripByID(ctx context.Context, id string) (*domain.TripModel, error) {
	return s.repo.GetTripByID(ctx, id)
}

func (s *service) UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error {
	return s.repo.UpdateTrip(ctx, tripID, status, driver)
}
