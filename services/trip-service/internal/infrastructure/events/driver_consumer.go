package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"ride-sharing/services/trip-service/internal/domain"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/messaging"
	"ride-sharing/shared/proto/driver"
	pb "ride-sharing/shared/proto/trip"

	amqp "github.com/rabbitmq/amqp091-go"
)

type driverConsumer struct {
	rabbitmq *messaging.RabbitMQ
	service  domain.TripService
}

func NewDriverConsumer(rabbitmq *messaging.RabbitMQ, service domain.TripService) *driverConsumer {
	return &driverConsumer{
		rabbitmq: rabbitmq,
		service:  service,
	}
}

func (c *driverConsumer) Listen() error {
	return c.rabbitmq.ConsumeMessages(messaging.DriverTripResponseQueue, func(ctx context.Context, msg amqp.Delivery) error {
		var message contracts.AmqpMessage
		if err := json.Unmarshal(msg.Body, &message); err != nil {
			return fmt.Errorf("message unmarshalling failed: %v", err)
		}

		var payload messaging.DriverTripResponseData

		if err := json.Unmarshal(message.Data, &payload); err != nil {
			return fmt.Errorf("message unmarshalling failed: %v", err)
		}

		log.Printf("driver received message: %+v", payload)

		switch msg.RoutingKey {
		case contracts.DriverCmdTripAccept:
			if err := c.handleTripAccepted(ctx, payload.TripID, payload.Driver); err != nil {
				log.Printf("error handling trip accepted: %v", err)
				return err
			}
		case contracts.DriverCmdTripDecline:
			if err := c.handleTripDecline(ctx, payload.TripID, payload.RiderID); err != nil {
				log.Printf("error handling trip decline: %v", err)
				return err
			}
		default:
			log.Printf("unknown driver response routing key: %s, payload: %+v", msg.RoutingKey, payload)
		}

		return nil
	})
}

func (c *driverConsumer) handleTripAccepted(ctx context.Context, tripID string, driver *driver.Driver) error {
	// fetch first
	trip, err := c.service.GetTripByID(ctx, tripID)
	if err != nil {
		return err
	}

	if trip == nil {
		return fmt.Errorf("trip not found")
	}

	// update the trip
	if err := c.service.UpdateTrip(ctx, tripID, "accepted", driver); err != nil {
		return err
	}

	trip, err = c.service.GetTripByID(ctx, tripID)
	if err != nil {
		return err
	}

	// Build a pb.Trip payload so the frontend gets canonical JSON keys (id, userID, driver, selectedFare, route)
	protoTrip := &pb.Trip{
		Id:           trip.ID.Hex(),
		SelectedFare: func() *pb.RideFare { if trip.RideFare != nil { return trip.RideFare.ToProto() }; return nil }(),
		Route:        func() *pb.Route { if trip.RideFare != nil && trip.RideFare.Route != nil { return trip.RideFare.Route.ToProto() }; return nil }(),
		Status:       trip.Status,
		UserID:       trip.UserId,
		Driver:       trip.Driver,
	}
	marshalledTrip, err := json.Marshal(protoTrip)
	if err != nil {
		return err
	}

	// Notify the rider that driver has been assigned
	log.Printf("TripService: publishing driver_assigned for rider=%s trip=%s", trip.UserId, trip.ID.Hex())
	if err := c.rabbitmq.PublishMessage(ctx, contracts.TripEventDriverAssigned, contracts.AmqpMessage{
		OwnerID: trip.UserId,
		Data:    marshalledTrip,
	}); err != nil {
		return err
	}
	log.Printf("TripService: published driver_assigned for rider=%s trip=%s", trip.UserId, trip.ID.Hex())

	// Request Payment Service to create a Stripe Checkout session
	cmd := messaging.PaymentTripResponseData{
		TripID:   trip.ID.Hex(),
		UserID:   trip.UserId,
		DriverID: driver.Id,
		Amount:   func() float64 { if trip.RideFare != nil { return trip.RideFare.TotalPriceCents }; return 0 }(),
		Currency: "USD",
	}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	log.Printf("TripService: publishing payment_cmd.create_session for rider=%s trip=%s", trip.UserId, trip.ID.Hex())
	if err := c.rabbitmq.PublishMessage(ctx, contracts.PaymentCmdCreateSession, contracts.AmqpMessage{
		OwnerID: trip.UserId,
		Data:    cmdBytes,
	}); err != nil {
		return err
	}
	log.Printf("TripService: published payment_cmd.create_session for rider=%s trip=%s", trip.UserId, trip.ID.Hex())

	return nil

	return nil
}

func (c *driverConsumer) handleTripDecline(ctx context.Context, tripID, riderID string) error {
	// When a driver declines, re-dispatch the trip to find another driver

	trip, err := c.service.GetTripByID(ctx, tripID)
	if err != nil {
		return err
	}
	if trip == nil {
		return fmt.Errorf("trip not found")
	}

	// Ensure trip remains open for matching
	if trip.Status != "pending" {
		if err := c.service.UpdateTrip(ctx, tripID, "pending", nil); err != nil {
			return err
		}
		trip, err = c.service.GetTripByID(ctx, tripID)
		if err != nil {
			return err
		}
	}

	// Build full trip payload required by driver-service (needs SelectedFare.PackageSlug)
	protoTrip := &pb.Trip{
		Id:           trip.ID.Hex(),
		SelectedFare: func() *pb.RideFare { if trip.RideFare != nil { return trip.RideFare.ToProto() }; return nil }(),
		Route:        func() *pb.Route { if trip.RideFare != nil && trip.RideFare.Route != nil { return trip.RideFare.Route.ToProto() }; return nil }(),
		Status:       trip.Status,
		UserID:       trip.UserId,
		Driver:       trip.Driver,
	}

	payload := messaging.TripEventData{Trip: protoTrip}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if err := c.rabbitmq.PublishMessage(ctx, contracts.TripEventDriverNotInterested, contracts.AmqpMessage{
		OwnerID: riderID,
		Data:    data,
	}); err != nil {
		return err
	}
	return nil
}
