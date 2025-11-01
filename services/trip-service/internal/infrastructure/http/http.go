package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"ride-sharing/services/trip-service/internal/domain"
	"ride-sharing/shared/types"
)

type previewTripRequest struct {
	UserID      string           `json:"userID"`
	Pickup      types.Coordinate `json:"pickup"`
	Destination types.Coordinate `json:"destination"`
}

type HttpHandler struct {
	Service domain.TripService
}

func (s *HttpHandler) HandleTripPreview(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var reqBody previewTripRequest

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	fare := &domain.RideFareModel{
		UserId: "2",
	}

	t, err := s.Service.CreateTrip(ctx, fare)
	if err != nil {
		log.Println(err)
		http.Error(w, "failed to create trip", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}
