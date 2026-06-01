package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/openchami/credential-manager/pkg/secrets"
)

func main() {
	masterKey := os.Getenv("MASTER_KEY")
	if masterKey == "" {
		log.Fatal("MASTER_KEY environment variable is required")
	}

	store, err := secrets.NewLocalSecretStore(masterKey, "secrets.json", true)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	payload := map[string]interface{}{
		"currentUsername": "root",
		"currentPassword": "initial0",
		"newPassword":     "new-pass",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload to JSON: %v", err)
	}

	err = store.StoreSecretByID("bmc-secret-172", string(payloadBytes))
	if err != nil {
		log.Fatalf("Failed to store secret: %v", err)
	}

	log.Println("Successfully seeded bmc-secret-172 into secrets.json")
}
