package main

import (
	"log"
	"os"

	// Update this import path to match your actual module name
	"github.com/user/credential-manager/internal/secrets"
)

func main() {
	masterKey := os.Getenv("MASTER_KEY")
	store, err := secrets.NewLocalSecretStore(masterKey, "/tmp/vault-secrets.json", true)
	if err != nil {
		log.Fatalf("Failed to init store: %v", err)
	}

	err = store.Store("curr-pass-001", map[string]interface{}{"value": "initial0"})
	if err != nil {
		log.Fatal(err)
	}

	err = store.Store("new-pass-001", map[string]interface{}{"value": "new-pass"})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Secrets seeded successfully to /tmp/vault-secrets.json")
}
