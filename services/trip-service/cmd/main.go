package main

import (
	"context"
	"log"
	"ride-sharing/services/trip-service/internal/domain"
	"ride-sharing/services/trip-service/internal/infrastructure/repository"
	"ride-sharing/services/trip-service/internal/service"
	"time"
)

func main() {
	log.Println("Starting trip Men")
	ctx := context.Background()
	inmemRepo := repository.NewInmemRepository()
	svc := service.NewTripService(inmemRepo)
	fare := domain.RideFareModel{
		UserId: "1",
	}
	t, err := svc.CreateTrip(ctx, &fare)
	if err != nil {
		log.Println(err)
	}
	log.Println(t)
	// keep running programming a run
	for {
		time.Sleep(time.Second)
	}
}
