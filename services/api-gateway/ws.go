package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/util"
)

// WebSocket upgrader with permissive origin check
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Rider WebSocket handler
func handleRidersWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("WebSocket error: missing userID query parameter")
		return
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
func handleDriversWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("WebSocket error: missing userID query parameter")
		return
	}

	packageSlug := r.URL.Query().Get("packageSlug")
	if packageSlug == "" {
		log.Println("WebSocket error: missing packageSlug query parameter")
		return
	}

	type Driver struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		ProfilePicture string `json:"profilePicture"`
		CarPlate       string `json:"carPlate"`
		PackageSlug    string `json:"packageSlug"`
	}

	msg := contracts.WSMessage{
		Type: "driver.cmd.register",
		Data: Driver{
			ID:             userID,
			Name:           "Amir",
			ProfilePicture: util.GetRandomAvatar(1),
			CarPlate:       "ABC123",
			PackageSlug:    packageSlug,
		},
	}

	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("WebSocket write error: %v", err)
		return
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
		log.Printf("Driver message from %s: %s", userID, message)
	}
}
