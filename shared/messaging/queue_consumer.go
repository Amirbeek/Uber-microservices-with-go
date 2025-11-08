package messaging

import (
	"encoding/json"
	"log"

	"ride-sharing/shared/contracts"
)

type QueueConsumer struct {
	rb        *RabbitMQ
	connMgr   *ConnectionManager
	queueName string
}

func NewQueueConsumer(rb *RabbitMQ, connMgr *ConnectionManager, queueName string) *QueueConsumer {
	return &QueueConsumer{
		rb:        rb,
		connMgr:   connMgr,
		queueName: queueName,
	}
}

func (qc *QueueConsumer) Start() error {
	msgs, err := qc.rb.Channel.Consume(
		qc.queueName,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for msg := range msgs {
			var msgBody contracts.AmqpMessage
			if err := json.Unmarshal(msg.Body, &msgBody); err != nil {
				log.Println("Failed to unmarshal message:", err)
				continue
			}

			userID := msgBody.OwnerID

			var payload any
			if msgBody.Data != nil {
				if err := json.Unmarshal(msgBody.Data, &payload); err != nil {
					log.Println("Failed to unmarshal payload:", err)
					continue
				}
			}

			clientMsg := contracts.WSMessage{
				Type: msg.RoutingKey,
				Data: payload,
			}

			log.Printf("QueueConsumer: delivering type=%s to OwnerID=%s via queue=%s", msg.RoutingKey, userID, qc.queueName)
			if err := qc.connMgr.SendMessage(userID, clientMsg); err != nil {
				log.Printf("QueueConsumer: failed to send type=%s to user=%s via queue=%s: %v", msg.RoutingKey, userID, qc.queueName, err)
			} else {
				log.Printf("QueueConsumer: sent type=%s to user=%s via queue=%s", msg.RoutingKey, userID, qc.queueName)
			}
		}
	}()

	return nil
}
