package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"ride-sharing/shared/contracts"
	messaging "ride-sharing/shared/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TripConsumer struct {
	rabbitmq *messaging.RabbitMQ
	service  *Service
}

func NewTripConsumer(rabbitmq *messaging.RabbitMQ, service *Service) *TripConsumer {
	return &TripConsumer{
		rabbitmq: rabbitmq,
		service:  service,
	}
}

func (c *TripConsumer) Listen() error {
	return c.rabbitmq.ConsumeMessages(messaging.FindAvailableDriverQueue, func(ctx context.Context, msg amqp.Delivery) error {
		var tripEvent contracts.AmqpMessage
		if err := json.Unmarshal(msg.Body, &tripEvent); err != nil {
			return fmt.Errorf("tripEvent unmarshalling failed: %v", err)
		}

		var payload messaging.TripEventData

		if err := json.Unmarshal(tripEvent.Data, &payload); err != nil {
			return fmt.Errorf("tripEvent unmarshalling failed: %v", err)
		}

		log.Printf("driver received message: %+v", payload)

		switch msg.RoutingKey {
		case contracts.TripEventCreated, contracts.TripEventDriverNotInterested:
			return c.handleFindAndNotifyDriver(ctx, payload)
		}

		log.Printf("unknown trip event: %+v", payload)
		return nil
	})
}

func (c *TripConsumer) handleFindAndNotifyDriver(ctx context.Context, payload messaging.TripEventData) error {
	suitableIDS := c.service.FindAvailableDrivers(payload.Trip.SelectedFare.PackageSlug)

	log.Printf("Found suitable Drivers: %d", len(suitableIDS))
	if len(suitableIDS) == 0 {
		// NOTIFY THE DRIVER THAT NO DRIVERS ARE AVAILABLE
		if err := c.rabbitmq.PublishMessage(ctx, contracts.TripEventNoDriversFound, contracts.AmqpMessage{
			OwnerID: payload.Trip.UserID,
		}); err != nil {
			log.Printf("Failed to publish Driver Notifications: %v", err)
			return err
		}
		return nil
	}

	randomIndex := rand.Intn(len(suitableIDS))
	suitableDriverID := suitableIDS[randomIndex]

	marchaledEvent, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	//NOTIFY THE DRIVER about a potential trip

	if err := c.rabbitmq.PublishMessage(ctx, contracts.DriverCmdTripRequest, contracts.AmqpMessage{
		OwnerID: suitableDriverID,
		Data:    marchaledEvent,
	}); err != nil {
		log.Printf("Failed to publish Driver Notifications: %v", err)
		return err
	}
	return nil
}
