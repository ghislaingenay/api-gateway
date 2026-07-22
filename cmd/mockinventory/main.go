// Command mockinventory is a minimal mock downstream service standing in
// for a real "inventory" service behind the gateway (FEAT-010). It exists
// only to demonstrate the gateway's full request flow against a second
// upstream in the local docker-compose environment.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type inventoryItem struct {
	ID       string `json:"id"`
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/inventory/{id}", getInventoryItemHandler)

	log.Printf("mockinventory: listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("mockinventory: %v", err)
	}
}

func getInventoryItemHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(inventoryItem{ID: id, SKU: "MOCK-SKU-" + id, Quantity: 42})
}
