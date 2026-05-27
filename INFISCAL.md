# Implementation Plan: Infisical Secret Store Integration

## 1. Objective
Implement an Infisical-backed adapter for the `SecretStore` interface defined in the `bmc-manager` service. This will allow the service to dynamically fetch and store BMC credentials securely using Infisical instead of the local fallback store.

## 2. Dependencies
* **Target Package:** `pkg/secrets` (where the `SecretStore` interface resides).
* **SDK:** Retrieve and install the official Infisical Go SDK. `go get github.com/infisical/go-sdk`

## 3. Implementation Directives

### A. Adapter Construction
1. Create a new file `pkg/secrets/infisical_store.go`.
2. Define an `InfisicalSecretStore` struct that implements the `SecretStore` interface. The struct must hold an instance of the Infisical client and store target workspace/environment variables.
3. Create a constructor function `NewInfisicalSecretStore() (*InfisicalSecretStore, error)`.

### B. Authentication
1. The constructor must initialize the Infisical client using Universal Auth (Machine Identities).
2. Extract the authentication credentials from the following environment variables:
   * `INFISICAL_CLIENT_ID`
   * `INFISICAL_CLIENT_SECRET`
   * `INFISICAL_PROJECT_ID`
   * `INFISICAL_ENVIRONMENT`
3. Return an error from the constructor if any of these variables are missing.

### C. Interface Mapping
Map the `SecretStore` interface methods to the corresponding Infisical SDK operations. The `id` argument passed to the interface methods will serve as the secret's key name in Infisical.
* `Read(id string)`: Map to the SDK method for fetching a secret by name. Return the secret's string value.
* `Write(id string, data string)`: Map to the SDK method for creating a new secret.
* `Update(id string, data string)`: Map to the SDK method for updating an existing secret.
* `Delete(id string)`: Map to the SDK method for deleting a secret.

### D. Configuration Factory
1. Locate the main application configuration/entry point (e.g., `cmd/server/main.go`).
2. Implement routing logic to inspect a `SECRET_STORE_BACKEND` environment variable at startup.
3. If `SECRET_STORE_BACKEND=infisical`, call `NewInfisicalSecretStore()` and inject it into the Reconciler dependency tree.
4. If empty or set to any other value, default to injecting the `LocalSecretStore`.

## 4. Verification Requirements
* **Unit Tests:** Write a table-driven test for `InfisicalSecretStore` in `pkg/secrets/infisical_store_test.go`.
* You must mock the Infisical client interface (or use the appropriate SDK mocking utilities) to verify that `Read`, `Write`, `Update`, and `Delete` correctly pass the `id` argument and the expected Project ID/Environment configurations to the SDK.