package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"ride-sharing/shared/env"
)

var (
	httpAddr = env.GetString("GATEWAY_HTTP_ADDR", ":8081")
)

func main() {
	log.Println("Starting API  Gateway")

	mux := http.NewServeMux()

	mux.HandleFunc("POST /trip/preview", enableCors(handleTripPreview))
	mux.HandleFunc("POST /trip/start", enableCors(handleTripStart))
	mux.HandleFunc("GET /ws/drivers", handleDriversWebSocket)
	mux.HandleFunc("GET /ws/riders", handleRidersWebSocket)

	server := &http.Server{Addr: httpAddr, Handler: mux}

	serverErrors := make(chan error, 1)
	go func() {
		log.Println("API Gateway Listening on ", httpAddr)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)

	select {
	case err := <-serverErrors:
		log.Printf("Error starting the server: %v \n", err)
	case sig := <-shutdown:
		log.Println("Shutting down the server... \n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Could not stop the server gracefully: %v \n", err)
			server.Close()
		}
	}
}
