# Handoff

## Summary of Implemented Business Logic
- Added a `SecretStore` abstraction (`pkg/secrets`) with `Read`, `Write`, `Update`, and `Delete` methods.
- Implemented `LocalSecretStore` as an in-memory fallback implementation.
- Implemented `BmcCredential` reconciliation logic in `pkg/reconcilers/bmccredential_reconciler.go`:
  - Reads current password via `SecretStore.Read(nodeIdentifier)`.
  - Generates a cryptographically secure random password.
  - Sends Redfish `PATCH` to `https://<address>/redfish/v1/AccountService/Accounts/<targetAccount>`.
  - Uses basic auth with target account username and current password.
  - Uses HTTP client transport with `InsecureSkipVerify: true` for self-signed BMC certs.
  - On `200` or `204`, persists new password via `SecretStore.Update(nodeIdentifier, newPassword)`.
  - Updates status on both success and failure paths and records UTC rotation timestamp.
- Wired reconciler with injected `SecretStore` from server startup (`cmd/server/main.go`).
- Added table-driven unit tests for success, unauthorized, and timeout reconciliation scenarios.

## Schema Fields

### Spec (`BmcCredentialSpec`)
- `address` (string): Target BMC hostname or IP.
- `targetAccount` (string): Target account ID/username on the BMC.
- `nodeIdentifier` (string): Unique node identifier used as the `SecretStore` key.

### Status (`BmcCredentialStatus`)
- `rotationSucceeded` (bool): Indicates whether rotation succeeded.
- `lastRotationUTC` (string): UTC timestamp of the last rotation attempt.
- `failureReason` (string): Failure reason message when rotation fails.

## Verified Server Startup Command
```bash
go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"
```

## Verified Successful Create Command
```bash
curl -sS -o /tmp/bmc_create_response.json -w "%{http_code}" -X POST "http://127.0.0.1:8080/bmccredentials/" -H "Content-Type: application/json" -d '{"metadata":{"name":"node-a"},"spec":{"address":"127.0.0.1","targetAccount":"admin","nodeIdentifier":"node-a"}}'
```

Expected/observed status code from this command: `201`
