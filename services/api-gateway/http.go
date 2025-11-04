package main

import (
	"encoding/json"
	"log"
	"net/http"
	"ride-sharing/services/api-gateway/grpc_clients"
	"ride-sharing/shared/contracts"
	pb "ride-sharing/shared/proto/trip"
)

func handleTripStart(w http.ResponseWriter, r *http.Request) {
	var body startTripRequest

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		log.Printf("trip client init error: %v", err)
		http.Error(w, "trip service unavailable", http.StatusBadGateway)
		return
	}
	defer tripService.Close()

	trip, err := tripService.Client.CreateTrip(r.Context(), &pb.CreateTripRequest{
		RideFareID: body.RideFareID,
		UserID:     body.UserID,
	})

	if err != nil {
		log.Printf("trip client create error: %v", err)
		http.Error(w, "trip service unavailable", http.StatusBadGateway)
		return
	}

	// Frontend expects a plain object { "tripID": string }
	writeJSON(w, http.StatusOK, struct{ TripID string `json:"tripID"` }{TripID: trip.GetTripID()})

}

func handleTripPreview(w http.ResponseWriter, r *http.Request) {
	var reqBody previewTripRequest

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if reqBody.UserID == "" {
		http.Error(w, "UserID is required", http.StatusBadRequest)
		return
	}

	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		log.Printf("trip client init error: %v", err)
		http.Error(w, "trip service unavailable", http.StatusBadGateway)
		return
	}
	defer tripService.Close()

	tripPreview, err := tripService.Client.PreviewTrip(r.Context(), reqBody.ToProto())
	if err != nil {
		log.Printf("PreviewTrip RPC error: %v", err)
		http.Error(w, "failed to preview trip", http.StatusBadGateway)
		return
	}

	response := contracts.APIResponse{Data: tripPreview}
	writeJSON(w, http.StatusOK, response)
}
