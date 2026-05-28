package main

import (
	"context"
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

	ctx := context.Background()

	err = store.StoreSecret(ctx, "curr-pass-001", "initial0")
	if err != nil {
		log.Fatal(err)
	}

	err = store.StoreSecret(ctx, "new-pass-001", "new-pass")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Secrets seeded successfully to /tmp/vault-secrets.json")
}
