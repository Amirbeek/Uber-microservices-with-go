package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"ride-sharing/shared/contracts"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	TripExchange = "trip"
)

type RabbitMQ struct {
	connection *amqp.Connection
	Channel    *amqp.Channel
}

func NewRabbitMQ(uri string) (*RabbitMQ, error) {
	// Retry to allow the broker to come up and avoid startup races
	var conn *amqp.Connection
	var ch *amqp.Channel
	var err error
	for attempt := 1; attempt <= 30; attempt++ { // ~60s with 2s sleep
		conn, err = amqp.Dial(uri)
		if err == nil {
			ch, err = conn.Channel()
			if err == nil {
				break
			}
			_ = conn.Close()
		}
		log.Printf("RabbitMQ not ready (attempt %d/30): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq after retries: %v", err)
	}

	rmq := &RabbitMQ{
		connection: conn,
		Channel:    ch,
	}

	// log returned (unroutable) messages when publishing with mandatory=true
	returns := rmq.Channel.NotifyReturn(make(chan amqp.Return, 1))
	go func() {
		for ret := range returns {
			log.Printf("RabbitMQ returned message: replyCode=%d replyText=%s exchange=%s routingKey=%s", ret.ReplyCode, ret.ReplyText, ret.Exchange, ret.RoutingKey)
		}
	}()

	if err := rmq.setupExchangesAndQueues(); err != nil {
		rmq.Close()
		return nil, fmt.Errorf("failed to set up RabbitMQ: %v", err)
	}
	return rmq, nil
}

func (r *RabbitMQ) PublishMessage(ctx context.Context, routingKey string, message contracts.AmqpMessage) error {
	log.Printf("publishing message with routing key %s", routingKey)

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	return r.Channel.PublishWithContext(ctx,
		TripExchange, // exchange
		routingKey,   // routing key
		true,         // mandatory - log returned if unroutable
		false,        // immediate (deprecated)
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         jsonMsg,
			DeliveryMode: amqp.Persistent,
		})
}

type MessageHandler func(context.Context, amqp.Delivery) error

func (r *RabbitMQ) ConsumeMessages(qName string, handler MessageHandler) error {
	// Qos(1, 0, false) - bu RabbitMQga shuni aytyapti:
	// "Bir consumer bir vaqtning o'zida faqat BIRTA xabar olsin."
	// Ya'ni consumer xabarni tugatib Ack qilmaguncha, RabbitMQ unga yangi xabar bermaydi.
	// Bu "Fair Dispatch" deb ataladi — sekin ishlaydigan consumerlar ham haddan tashqari yuklanmaydi.

	err := r.Channel.Qos(1, 0, false)
	if err != nil {
		// Agar QoS o'rnatishda xatolik bo'lsa — uni qaytaramiz
		return fmt.Errorf("failed to set QoS: %v", err)
	}

	msgs, err := r.Channel.Consume(
		qName, // queue
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for msg := range msgs {
			log.Printf("Received a message: %s", msg.Body)
			if err := handler(ctx, msg); err != nil {
				log.Printf("ERROR: Failed to handle message: %v. Message body: %s", err, msg.Body)
				// Nack the message. Set requeue to false to avoid immediate redelivery loops.
				// Consider a dead-letter exchange (DLQ) or a more sophisticated retry mechanism for production.

				if nackErr := msg.Nack(false, false); nackErr != nil {
					log.Printf("ERROR: Failed to Nack message: %v", nackErr)
				}

				// Continue to the next message
				continue
			}

			// Only Ack if the handler succeeds
			if ackErr := msg.Ack(false); ackErr != nil {
				log.Printf("ERROR: Failed to Ack message: %v. Message body: %s", ackErr, msg.Body)
			}

		}
	}()
	return nil
}

func (r *RabbitMQ) setupExchangesAndQueues() error {
	// create exchange
	err := r.Channel.ExchangeDeclare(
		TripExchange, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)

	if err != nil {
		return fmt.Errorf("failed to set Exchange declare: %v", err)
	}

	// Queue: find_available_drivers -> trip.event.created, trip.event.driver_not_interested
	if err := r.declareAndBindQueue(
		FindAvailableDriverQueue,
		[]string{
			contracts.TripEventCreated,
			contracts.TripEventDriverNotInterested,
		},
		TripExchange,
	); err != nil {
		return fmt.Errorf("failed to declare and bind queue: %v", err)
	}

	// Queue: driver_cmd_trip_request -> driver.cmd.trip_request (consumed by API Gateway/Drivers)
	if err := r.declareAndBindQueue(
		DriverCmdTripRequestQueue,
		[]string{
			contracts.DriverCmdTripRequest,
		},
		TripExchange,
	); err != nil {
		return fmt.Errorf("failed to declare and bind queue: %v", err)
	}

	// Queue: driver_trip_response -> driver.cmd.trip_accept, driver.cmd.trip_decline (consumed by Trip Service)
	if err := r.declareAndBindQueue(
		DriverTripResponseQueue,
		[]string{
			contracts.DriverCmdTripAccept,
			contracts.DriverCmdTripDecline,
		},
		TripExchange,
	); err != nil {
		return fmt.Errorf("failed to declare and bind queue: %v", err)
	}

	// Queue: notify_driver_no_drivers_found -> trip.event.no_drivers_found (consumed by API Gateway/Riders)
	if err := r.declareAndBindQueue(
		NotifyDriverNoDriversFoundQueue,
		[]string{
			contracts.TripEventNoDriversFound,
		},
		TripExchange,
	); err != nil {
		return fmt.Errorf("failed to declare and bind queue: %v", err)
	}

	// Queue: notify_driver_assign -> trip.event.driver_assigned (consumed by API Gateway/Riders)
	if err := r.declareAndBindQueue(
		NotifyDriverAssignedQueue,
		[]string{
			contracts.TripEventDriverAssigned,
		},
		TripExchange,
	); err != nil {
		return fmt.Errorf("failed to declare and bind queue: %v", err)
	}

	return nil
}

func (r *RabbitMQ) declareAndBindQueue(queueName string, messageTypes []string, exchange string) error {
	q, err := r.Channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %v", err)
	}

	for _, routingKey := range messageTypes {
		if err = r.Channel.QueueBind(
			q.Name,     // queue
			routingKey, // routing key
			exchange,   // exchange
			false,      // no-wait
			nil,        // arguments
		); err != nil {
			return fmt.Errorf("failed to bind queue: %v", err)
		}
	}

	return nil
}

func (r *RabbitMQ) Close() {
	if r.connection != nil {
		r.connection.Close()
	}
	if r.Channel != nil {
		r.Channel.Close()
	}
}
