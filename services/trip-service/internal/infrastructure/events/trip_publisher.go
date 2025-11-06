package events

import (
	"context"
	"encoding/json"
	"ride-sharing/services/trip-service/internal/domain"
	"ride-sharing/shared/contracts"
	messaging "ride-sharing/shared/messaging"
	pb "ride-sharing/shared/proto/trip"
)

type TripEventPublisher struct {
	rabbitmq *messaging.RabbitMQ
}

func NewTripEventPublisher(rabbitmq *messaging.RabbitMQ) *TripEventPublisher {
	return &TripEventPublisher{rabbitmq}
}

func (p *TripEventPublisher) PublishTripCreated(ctx context.Context, trip *domain.TripModel) error {
	protoTrip := &pb.Trip{
		Id:           trip.ID.Hex(),
		SelectedFare: trip.RideFare.ToProto(),
		Route: func() *pb.Route {
			if trip.RideFare != nil && trip.RideFare.Route != nil {
				return trip.RideFare.Route.ToProto()
			}
			return nil
		}(),
		Status: trip.Status,
		UserID: trip.UserId,
		Driver: trip.Driver,
	}
	payload := messaging.TripEventData{Trip: protoTrip}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.rabbitmq.PublishMessage(ctx, contracts.TripEventCreated, contracts.AmqpMessage{
		OwnerID: trip.UserId,
		Data:    data,
	})
}
