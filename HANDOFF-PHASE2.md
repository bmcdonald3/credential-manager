# Phase 2 Handoff: Cryptographic Integration and Execution

## Summary of Implemented Changes

### 1. Secure Storage Integration
- Added new package at `internal/secrets`:
  - `internal/secrets/encryption.go`
  - `internal/secrets/localStoreAdapter.go`
- Implemented:
  - AES-GCM encryption/decryption
  - HKDF (SHA-256) key derivation per secret with random salt
  - Master key validation from 64-char hex (32 bytes)
  - JSON-backed local secret document adapter
  - Secret lookup and secret storage helper methods

### 2. Server Wiring and Configuration
- Updated server startup in `cmd/server/main.go` to:
  - Accept `MASTER_KEY` via env (required)
  - Accept secrets file via `--secrets-file` and `SECRETS_FILE` (default `/tmp/vault-secrets.json`)
  - Initialize local store before reconciler startup using:
    - `secrets.NewLocalSecretStore(masterKey, secretsFile, true)`
  - Create HTTP client with TLS verification disabled (`InsecureSkipVerify: true`)
  - Inject both dependencies into reconciler registration

### 3. Reconciler Dependency Injection
- Updated registration flow in `pkg/reconcilers/registration_generated.go` to pass:
  - Secret store resolver
  - HTTP client dependency
- Updated `pkg/reconcilers/bmccredential_reconciler_generated.go` to hold injected dependencies.
- Added constructor in `pkg/reconcilers/bmccredential_reconciler.go` to wire dependencies.

### 4. Runtime Reconciliation Logic
- Implemented in `pkg/reconcilers/bmccredential_reconciler.go`:
  - Validation gate remains for secret ID fields.
  - Secret resolution:
    - Lookup/decrypt current and new passwords via injected LocalStore.
    - On lookup/decrypt failure, Status set to Failed with reason.
  - Redfish execution:
    - PATCH request to:
      - `https://[TargetAddress]/redfish/v1/AccountService/Accounts/[TargetAccount]`
    - Basic Auth with:
      - Username from `CurrentUsername`
      - Password from decrypted `currentPasswordSecretID`
    - JSON payload:
      - `{"Password":"<decrypted_new_password>"}`
    - TLS verification disabled through configured transport.
  - Status behavior:
    - 200 or 204 => State = Success, reason cleared
    - request error / timeout / non-success HTTP (incl. 401) => State = Failed with HTTP/error reason
    - `LastUpdatedAtUTC` is set on outcomes

### 5. Unit Test Coverage
- Expanded tests in `pkg/reconcilers/bmccredential_reconciler_test.go` using mocked dependencies:
  - Secret resolution/decryption failure path updates Failed state/reason
  - HTTP unauthorized path updates Failed state/reason
  - Successful 204 path updates Success and verifies URL/auth/payload

## Dependencies
- Confirmed and tidied with `go mod tidy`:
  - `golang.org/x/crypto` (HKDF)
  - `github.com/mitchellh/mapstructure` (secrets JSON decoding)

## Verification Commands Run

### Compile and Unit Tests
- `gofmt -w internal/secrets/encryption.go internal/secrets/localStoreAdapter.go cmd/server/main.go pkg/reconcilers/bmccredential_reconciler.go pkg/reconcilers/bmccredential_reconciler_generated.go pkg/reconcilers/registration_generated.go pkg/reconcilers/bmccredential_reconciler_test.go`
- `go mod tidy`
- `go build ./...`
- `go test ./...`

### Integration Validation Run
- Export master key and start server:
  - `MASTER_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1" --secrets-file="/tmp/vault-secrets.json"`
- Seed encrypted mock secrets in `/tmp/vault-secrets.json` with IDs:
  - `secret-current-001`
  - `secret-new-001`
- Trigger reconciler:
  - `curl -s -o /tmp/phase2_create_response.json -w "%{http_code}" -X POST http://localhost:8080/bmccredentials/ -H "Content-Type: application/json" -d '{"metadata":{"name":"rotate-admin-phase2"},"spec":{"targetAddress":"127.0.0.1:9443","currentUsername":"admin","currentPasswordSecretID":"secret-current-001","targetAccount":"admin","newPasswordSecretID":"secret-new-001"}}'`
- Observed result:
  - Resource create returned HTTP 201.
  - Reconciler logs showed secret resolution and attempted Redfish PATCH call to `https://127.0.0.1:9443/redfish/v1/AccountService/Accounts/admin`.
  - In this validation run, network call failed with connection refused (expected in local mock environment without a Redfish endpoint).

## Notes
- The implementation records execution and cryptographic failures in `status.validationFailureReason` and marks `status.state` as Failed.
- Success clears the failure reason and marks state as Success.
