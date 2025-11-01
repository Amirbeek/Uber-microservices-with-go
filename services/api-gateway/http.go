package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"ride-sharing/shared/contracts"
)

func handleTripPreview(w http.ResponseWriter, r *http.Request) {
	var reqBody previewTripRequest

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if reqBody.UserID == "" {
		http.Error(w, "UserID is required", http.StatusBadRequest)
		return
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		http.Error(w, "failed to encode JSON", http.StatusInternalServerError)
		return
	}

	resp, err := http.Post("http://trip-service:8083/preview", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		http.Error(w, "failed to make request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var tripResp any
	if err := json.NewDecoder(resp.Body).Decode(&tripResp); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	response := contracts.APIResponse{Data: tripResp}
	writeJSON(w, http.StatusOK, response)
}
