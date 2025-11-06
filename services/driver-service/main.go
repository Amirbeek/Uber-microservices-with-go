package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"ride-sharing/shared/env"
	message "ride-sharing/shared/messaging"
	"syscall"

	grpcserver "google.golang.org/grpc"
)

var (
	GrpcAddr = env.GetString("GRPC_ADDR", ":9092")
)

func main() {
	rabbitmqURI := env.GetString("RABBITMQ_URI", "amqp://guest:guest@rabbitmq:5672/")
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
	service := NewService()
	grpcServer := grpcserver.NewServer()
	NewGrpcHandler(grpcServer, service)

	conn, err := message.NewRabbitMQ(rabbitmqURI)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer conn.Close()
	consumer := NewTripConsumer(conn, service)
	go func() {
		if err := consumer.Listen(); err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
	}()
	log.Println("Starting gRPC server Driver service on port: " + lis.Addr().String())

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
			cancel()
		}
	}()

	// wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down gRPC server DriverF service")
	grpcServer.GracefulStop()

}
