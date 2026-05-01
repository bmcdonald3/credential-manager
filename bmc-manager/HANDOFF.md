# Handoff: BMC Manager

## Business Logic Implemented
- Added custom reconciliation for `BmcCredential` in `pkg/reconcilers/bmccredential_reconciler.go`.
- On each reconcile (create/update/periodic), the reconciler builds:
  - `https://<spec.address>/redfish/v1/`
- It executes an HTTP `GET` with Basic Auth from `spec.username` and `spec.password`.
- Status behavior:
  - Always sets `status.lastCheckedAt` to current UTC time.
  - If HTTP returns `200 OK`: sets `status.verified=true` and clears `status.failureReason`.
  - If HTTP request fails, times out, or returns non-200: sets `status.verified=false` and populates `status.failureReason`.

## Final Schema

### BmcCredentialSpec
- `address` (string, required): BMC hostname or IP address.
- `username` (string, required): credential username.
- `password` (string, required): credential password.

Validation:
- `address` must be a valid hostname or IP address.
- `username` and `password` must be non-empty.

### BmcCredentialStatus
- `verified` (bool): true if latest Redfish verification succeeded.
- `lastCheckedAt` (time): UTC timestamp of latest verification attempt.
- `failureReason` (string): error details for latest failed verification.

## Reviewer Verification Commands
Run from project root (`bmc-manager`):

```bash
go mod tidy
go build ./...
go test ./...
```

Run targeted reconciliation tests only:

```bash
go test ./pkg/reconcilers -run TestReconcileBmcCredential_Verification -v
```
