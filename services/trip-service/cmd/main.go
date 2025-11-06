package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"ride-sharing/services/trip-service/internal/infrastructure/events"
	"ride-sharing/services/trip-service/internal/infrastructure/grpc"
	"ride-sharing/services/trip-service/internal/infrastructure/repository"
	"ride-sharing/services/trip-service/internal/service"
	"ride-sharing/shared/env"
	message "ride-sharing/shared/messaging"
	"syscall"

	grpcserver "google.golang.org/grpc"
)

var (
	GrpcAddr = env.GetString("GRPC_ADDR", ":9093")
)

func main() {
	rabbitmqURI := env.GetString("RABBITMQ_URI", "amqp://guest:guest@rabbitmq:5672/")
	inmemRepo := repository.NewInmemRepository()
	svc := service.NewTripService(inmemRepo)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		cancel()
	}()

	lis, err := net.Listen("tcp", GrpcAddr)

	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	//RabbitMq connection
	rabbitMQ, err := message.NewRabbitMQ(rabbitmqURI)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer rabbitMQ.Close()

	publisher := events.NewTripEventPublisher(rabbitMQ)
	driverConsumer := events.NewDriverConsumer(rabbitMQ, svc)
	go driverConsumer.Listen()

	log.Println("Connected to rabbitmq with Rider Server")

	grpcServer := grpcserver.NewServer()
	grpc.NewGRPCHandler(grpcServer, svc, publisher)

	log.Println("Starting gRPC server Trip service on port: " + lis.Addr().String())

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
			cancel()
		}
	}()

	// wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down gRPC server Trip service")
	grpcServer.GracefulStop()

}
