# HANDOFF

## Business Logic Implemented
- Added custom reconciliation logic for `BmcCredential` that performs a Redfish password rotation request on create/update reconcile events.
- Reconciler sends an HTTP `PATCH` request to:
  - `https://[targetAddress]/redfish/v1/AccountService/Accounts/[targetAccount]`
- Request uses basic auth with `currentUsername` / `currentPassword` and JSON payload:
  - `{"Password":"<newPassword>"}`
- HTTP client is configured with TLS certificate verification disabled (`InsecureSkipVerify: true`) and a request timeout.
- Status behavior:
  - Always sets `lastRotationAttempt` to current UTC time on each attempt.
  - On HTTP `200` or `204`, sets `rotationSucceeded=true` and clears `failureReason`.
  - On request failure, timeout, unauthorized, or any non-2xx response, sets `rotationSucceeded=false` and stores the exact error text in `failureReason`.

## Exact Schema Fields

### Spec (`BmcCredentialSpec`)
- `targetAddress` (string, required)
- `currentUsername` (string, required)
- `currentPassword` (string, required)
- `targetAccount` (string, required)
- `newPassword` (string, required)

### Status (`BmcCredentialStatus`)
- `rotationSucceeded` (boolean)
- `lastRotationAttempt` (timestamp, UTC, nullable)
- `failureReason` (string)

## Verified Server Startup Command
```bash
go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"
```

## Verified Successful Curl Command
```bash
curl -sS -o /tmp/bmc_create_resp.json -w "%{http_code}" \
  -X POST http://127.0.0.1:8080/bmccredentials/ \
  -H "Content-Type: application/json" \
  -d '{"metadata":{"name":"rotate-admin"},"spec":{"targetAddress":"192.0.2.10","currentUsername":"admin","currentPassword":"old-pass","targetAccount":"2","newPassword":"new-pass"}}'
```
- Verified response status: `201`
