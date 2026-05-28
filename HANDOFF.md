# HANDOFF: BMC Manager Phase 1

## Summary
Implemented Phase 1 schema and service scaffolding for `BmcCredential` using Fabrica with Ent + SQLite, reconciliation enabled, and event/storage support.

Custom reconciliation logic in `pkg/reconcilers/bmccredential_reconciler.go` now validates that both secret ID fields are non-empty during reconcile (create/update trigger path), and updates status accordingly:
- Valid inputs: `state = "Validated"`, clears failure reason
- Invalid inputs: `state = "Failed"`, sets validation failure reason
- Always updates `lastUpdatedAtUTC` to current UTC time

No decryption, LocalStore lookup, or Redfish network actions were implemented (as required for this phase).

## Implemented Spec Fields
`apis/example.fabrica.dev/v1/bmccredential_types.go`

### BmcCredentialSpec
- `targetAddress` (string)
- `currentUsername` (string)
- `currentPasswordSecretID` (string)
- `targetAccount` (string)
- `newPasswordSecretID` (string)

### BmcCredentialStatus
- `state` (string)
- `lastUpdatedAtUTC` (time.Time)
- `validationFailureReason` (string)

## Reconciliation Validation Behavior
Validation performed in:
- `pkg/reconcilers/bmccredential_reconciler.go`

Validation rule:
- `currentPasswordSecretID` must not be empty/whitespace
- `newPasswordSecretID` must not be empty/whitespace

Status transitions:
- Pass -> `Validated` + clear failure reason
- Fail -> `Failed` + set error message
- Always set `lastUpdatedAtUTC = time.Now().UTC()`

Unit tests (table-driven):
- `pkg/reconcilers/bmccredential_reconciler_test.go`

## Verified Server Startup Command
```bash
export GOROOT="$HOME/sdk/go1.26.3" && export PATH="$GOROOT/bin:$PATH" && go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"
```

## Verified Successful Create Command (2xx)
```bash
curl -s -o /tmp/bmc_create_response.json -w "%{http_code}" -X POST http://localhost:8080/bmccredentials/ -H "Content-Type: application/json" -d '{"metadata":{"name":"rotate-admin-1"},"spec":{"targetAddress":"192.168.1.50","currentUsername":"admin","currentPasswordSecretID":"secret-current-001","targetAccount":"admin","newPasswordSecretID":"secret-new-001"}}'
```

Observed HTTP status: `201`
