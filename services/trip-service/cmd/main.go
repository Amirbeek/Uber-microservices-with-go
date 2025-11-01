package main

import (
	"log"
	"net/http"
	h "ride-sharing/services/trip-service/internal/infrastructure/http"
	"ride-sharing/services/trip-service/internal/infrastructure/repository"
	"ride-sharing/services/trip-service/internal/service"
	"ride-sharing/shared/env"
)

var (
	httpAddr = env.GetString("HTTP_ADDR", ":8083")
)

func main() {
	log.Println("Starting Trip API server")
	inmemRepo := repository.NewInmemRepository()
	svc := service.NewTripService(inmemRepo)

	mux := http.NewServeMux()

	httphandler := h.HttpHandler{Service: svc}
	mux.HandleFunc("POST /preview", httphandler.HandleTripPreview)

	server := &http.Server{Addr: httpAddr, Handler: mux}

	if err := server.ListenAndServe(); err != nil {
		log.Printf("Trip HTTP Error starting server: %s", err)
	}
}
