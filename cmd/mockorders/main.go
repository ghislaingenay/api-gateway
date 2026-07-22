// Command mockorders is a minimal mock downstream service standing in for
// a real "orders" service behind the gateway (FEAT-010). It exists only to
// demonstrate the gateway's full request flow (auth, rate limiting, caching,
// validation, proxying) against config/routes.json's orders-service routes
// in the local docker-compose environment.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type order struct {
	ID            string `json:"id"`
	CustomerEmail string `json:"customer_email,omitempty"`
	Quantity      int    `json:"quantity,omitempty"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/orders/{id}", getOrderHandler)
	mux.HandleFunc("POST /api/orders", createOrderHandler)

	log.Printf("mockorders: listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("mockorders: %v", err)
	}
}

func getOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writeJSON(w, http.StatusOK, order{
		ID:        id,
		Status:    "shipped",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func createOrderHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerEmail string `json:"customer_email"`
		Quantity      int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, order{
		ID:            strings.ReplaceAll(time.Now().UTC().Format("20060102T150405.000000000"), ".", ""),
		CustomerEmail: req.CustomerEmail,
		Quantity:      req.Quantity,
		Status:        "created",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
