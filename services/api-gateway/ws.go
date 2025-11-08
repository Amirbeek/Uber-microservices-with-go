package main

import (
	"encoding/json"
	"log"
	"net/http"
	"ride-sharing/services/api-gateway/grpc_clients"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/messaging"
	driver "ride-sharing/shared/proto/driver"

	"github.com/gorilla/websocket"
)

var (
	connManager = messaging.NewConnectionManager()
)

// WebSocket upgrader with permissive origin check
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Rider WebSocket handler
func handleRidersWebSocket(w http.ResponseWriter, r *http.Request, rabbitmq *messaging.RabbitMQ) {
	conn, err := connManager.Upgrade(w, r)
	if err != nil {
		log.Printf("Failed to set websocket upgrade: %+v\n", err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("WebSocket error: missing userID query parameter")
		return
	}

	connManager.Add(userID, conn)
	defer connManager.Remove(userID)

	// Initialize queue consumers
	queues := []string{
		messaging.NotifyDriverNoDriversFoundQueue,
		messaging.NotifyDriverAssignQueue,
		messaging.NotifyPaymentSessionCreatedQueue,
	}

	log.Printf("Rider WS subscribed user=%s to queues=%v", userID, queues)
	for _, q := range queues {
		consumer := messaging.NewQueueConsumer(rabbitmq, connManager, q)
		if err := consumer.Start(); err != nil {
			log.Printf("WebSocket start error: %v", err)
		}
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
		log.Printf("Rider message from %s: %s", userID, message)
	}
}

// Driver WebSocket handler
func handleDriversWebSocket(w http.ResponseWriter, r *http.Request, rabbitmq *messaging.RabbitMQ) {
	conn, err := connManager.Upgrade(w, r)
	if err != nil {
		log.Printf("Failed to set websocket upgrade: %+v\n", err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("WebSocket error: missing userID")
		return
	}

	packageSlug := r.URL.Query().Get("packageSlug")
	if packageSlug == "" {
		log.Println("WebSocket error: missing packageSlug")
		return
	}
	connManager.Add(userID, conn)

	ctx := r.Context()

	driverService, err := grpc_clients.NewDriverServiceClient(nil)
	if err != nil {
		log.Printf("gRPC driver service connection error: %v", err)
		return
	}
	defer driverService.Close()

	// ensure driver unregisters on exit
	defer func() {
		defer connManager.Remove(userID)

		_, unregErr := driverService.Client.UnregisterDriver(ctx, &driver.RegisterDriverRequest{
			DriverID: userID,
		})
		if unregErr != nil {
			log.Printf("UnregisterDriver error for %s: %v", userID, unregErr)
		} else {
			log.Printf("Driver unregistered: %s", userID)
		}
	}()

	// register driver
	driverData, err := driverService.Client.RegisterDriver(ctx, &driver.RegisterDriverRequest{
		DriverID:    userID,
		PackageSlug: packageSlug,
	})
	if err != nil {
		log.Printf("RegisterDriver error: %v", err)
		return
	}

	if err := connManager.SendMessage(userID, contracts.WSMessage{
		Type: contracts.DriverCmdRegister,
		Data: driverData.Driver,
	}); err != nil {
		log.Printf("WebSocket write error: %v", err)
		return
	}

	// Initialize queue consumers
	queues := []string{
		messaging.DriverCmdTripRequestQueue,
		messaging.NotifyDriverAssignQueue,
	}

	log.Printf("Driver WS subscribed user=%s to queues=%v", userID, queues)
	for _, q := range queues {
		consumer := messaging.NewQueueConsumer(rabbitmq, connManager, q)
		if err := consumer.Start(); err != nil {
			log.Printf("WebSocket start error: %v", err)
		}
	}

	// receive messages
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		type driverMessage = contracts.WSDriverMessage
		var driverMsg driverMessage

		if err := json.Unmarshal(raw, &driverMsg); err != nil {
			log.Printf("WebSocket unmarshal error: %v", err)
			continue
		}

		switch driverMsg.Type {
		case contracts.DriverCmdLocation:
			// Handle driver location
			continue
		case contracts.DriverCmdTripAccept, contracts.DriverCmdTripDecline:
			log.Printf("Driver WS forwarding type=%s from user=%s", driverMsg.Type, userID)
			if err := rabbitmq.PublishMessage(ctx, driverMsg.Type, contracts.AmqpMessage{
				OwnerID: userID,
				Data:    driverMsg.Data, // pass raw JSON bytes
			}); err != nil {
				log.Printf("WebSocket publish error: %v", err)
			} else {
				log.Printf("Driver WS forwarded type=%s from user=%s", driverMsg.Type, userID)
			}
		default:
			log.Printf("WebSocket unknown type: %s", driverMsg.Type)
		}
	}
}
