package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"ride-sharing/services/api-gateway/grpc_clients"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/env"
	message "ride-sharing/shared/messaging"
	pb "ride-sharing/shared/proto/trip"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
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
	writeJSON(w, http.StatusOK, struct {
		TripID string `json:"tripID"`
	}{TripID: trip.GetTripID()})

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

func handleStripeWebhook(w http.ResponseWriter, r *http.Request, rb *message.RabbitMQ) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	webhookKey := env.GetString("STRIPE_WEBHOOK_KEY", "")
	if webhookKey == "" {
		log.Printf("Webhook key is required")
		return
	}

	event, err := webhook.ConstructEventWithOptions(
		body,
		r.Header.Get("Stripe-Signature"),
		webhookKey,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)
	if err != nil {
		log.Printf("Error verifying webhook signature: %v", err)
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	log.Printf("Received Stripe event: %v", event)

	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession

		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			log.Printf("Error parsing webhook JSON: %v", err)
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		payload := message.PaymentStatusUpdateData{
			TripID:   session.Metadata["trip_id"],
			UserID:   session.Metadata["user_id"],
			DriverID: session.Metadata["driver_id"],
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error marshalling payload: %v", err)
			http.Error(w, "Failed to marshal payload", http.StatusInternalServerError)
			return
		}

		message := contracts.AmqpMessage{
			OwnerID: session.Metadata["user_id"],
			Data:    payloadBytes,
		}

		if err := rb.PublishMessage(
			r.Context(),
			contracts.PaymentEventSuccess,
			message,
		); err != nil {
			log.Printf("Error publishing payment event: %v", err)
			http.Error(w, "Failed to publish payment event", http.StatusInternalServerError)
			return
		}
	}

	// Explicit 200 OK so Stripe treats the webhook as delivered
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
