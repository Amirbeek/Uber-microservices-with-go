package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"ride-sharing/services/payment-service/internal/infrastructure/events"
	"ride-sharing/services/payment-service/internal/infrastructure/stripe"
	"ride-sharing/services/payment-service/internal/service"
	"syscall"

	"ride-sharing/services/payment-service/pkg/types"
	"ride-sharing/shared/env"
	"ride-sharing/shared/messaging"
)

var GrpcAddr = env.GetString("GRPC_ADDR", ":9004")

func main() {
	rabbitMqURI := env.GetString("RABBITMQ_URI", "amqp://guest:guest@rabbitmq:5672/")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	appURL := env.GetString("APP_URL", "http://localhost:3000")

	// Stripe config
	stripeCfg := &types.PaymentConfig{
		StripeSecretKey: env.GetString("STRIPE_SECRET_KEY", ""),
		SuccessURL:      env.GetString("STRIPE_SUCCESS_URL", appURL+"?payment=success"),
		CancelURL:       env.GetString("STRIPE_CANCEL_URL", appURL+"?payment=cancel"),
	}

	if stripeCfg.StripeSecretKey == "" {
		log.Fatalf("STRIPE_SECRET_KEY is not set")
		return
	}
	// Stripe processor
	paymentProcessor := stripe.NewStripeClient(stripeCfg)

	srv := service.NewPaymentService(paymentProcessor)

	// Trip Consumer
	log.Println(srv)
	// RabbitMQ connection
	rabbitmq, err := messaging.NewRabbitMQ(rabbitMqURI)
	if err != nil {
		log.Fatal(err)
	}
	defer rabbitmq.Close()
	tripConsumer := events.NewTripConsumer(rabbitmq, srv)
	go tripConsumer.Listen()

	log.Println("Starting RabbitMQ connection")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down payment service...")
}
