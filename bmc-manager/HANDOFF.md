# HANDOFF: BMC Manager

## Implemented Business Logic
The `BmcCredential` reconciler now performs password rotation against Redfish when resources are created or updated:

1. Reads `targetAddress`, current credentials, `targetAccount`, and `newPassword` from Spec.
2. Sends an HTTP `PATCH` to:
   - `https://<targetAddress>/redfish/v1/AccountService/Accounts/<targetAccount>`
3. Uses basic auth with the **current** username/password.
4. Sends JSON payload:
   - `{"Password":"<newPassword>"}`
5. Uses an HTTP client configured with TLS certificate verification disabled (`InsecureSkipVerify: true`) for self-signed BMC certs.
6. Updates status on every attempt, including UTC timestamp.

Status behavior:
- Success (`200 OK` or `204 No Content`):
  - `rotationSucceeded=true`
  - `failureReason=""`
- Failure (request error, `401 Unauthorized`, timeout, other non-200/204):
  - `rotationSucceeded=false`
  - `failureReason=<exact error string>`
- Always updates:
  - `lastRotationAttempt=<current UTC timestamp>`

## Schema Fields
### `BmcCredentialSpec`
- `targetAddress` (string, required)
- `currentUsername` (string, required)
- `currentPassword` (string, required)
- `targetAccount` (string, required; account ID or username)
- `newPassword` (string, required)

### `BmcCredentialStatus`
- `rotationSucceeded` (bool)
- `lastRotationAttempt` (timestamp, UTC)
- `failureReason` (string)
- `phase` (string)
- `message` (string)
- `ready` (bool)

## Verification Commands
### Build and tests
```bash
go mod tidy
go build ./...
go test ./...
```

### Focused reconciliation logic test
```bash
go test ./pkg/reconcilers -run TestRotateBMCPasswordWithTimeout -v
```

### Optional API-level exercise
Start the server:
```bash
go run ./cmd/server
```

Then create a `BmcCredential` object (adjust URL and payload to your local setup):
```bash
curl -X POST http://localhost:8080/api/example.fabrica.dev/v1/bmccredentials \
  -H 'Content-Type: application/json' \
  -d '{
    "apiVersion":"example.fabrica.dev/v1",
    "kind":"BmcCredential",
    "metadata":{"name":"node-1-cred-rotate"},
    "spec":{
      "targetAddress":"192.0.2.10",
      "currentUsername":"admin",
      "currentPassword":"oldSecret!",
      "targetAccount":"2",
      "newPassword":"newSecret!"
    }
  }'
```
