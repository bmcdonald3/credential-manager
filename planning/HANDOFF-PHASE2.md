## 1. Reconciliation Logic Summary

Phase 2 replaces inline credential fields on `BmcCredentialSpec` with a required `SecretID`. During reconciliation, the controller now validates `MASTER_KEY` and the required spec fields, opens `secrets.json` through `secrets.NewLocalSecretStore`, loads and decrypts the referenced secret payload, unmarshals `currentUsername`, `currentPassword`, and `newPassword`, and issues the Redfish password PATCH over HTTPS with TLS verification disabled.

The reconciler classifies missing `MASTER_KEY`, missing `secrets.json`, missing secrets, and invalid decrypted JSON as terminal failures. Those update status and halt retries by returning `nil`. Network errors and BMC `5xx` responses remain transient and are returned so Fabrica's default exponential backoff can retry them. Successful reconciliations clear `FailureReason`, set `RotationSucceeded = true`, stamp `LastRotationAttempt`, and record the observed trigger for idempotency.

## 2. Final Go Struct Definitions

```go
type BmcCredentialSpec struct {
    TargetAddress   string `json:"targetAddress" validate:"required"`
    TargetAccount   string `json:"targetAccount" validate:"required"`
    SecretID        string `json:"secretId" validate:"required"`
    RotationTrigger string `json:"rotationTrigger,omitempty"`
}

type BmcCredentialStatus struct {
    LastRotationAttempt  *time.Time `json:"lastRotationAttempt,omitempty"`
    RotationSucceeded    bool       `json:"rotationSucceeded"`
    FailureReason        string     `json:"failureReason,omitempty"`
    ObservedTriggerValue string     `json:"observedTriggerValue,omitempty"`
}
```

## 3. Verified Server Startup Command

```bash
export GOROOT="$HOME/sdk/go1.26.3" && export PATH="$GOROOT/bin:$PATH" && export MASTER_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef && go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"
```

## 4. Verified Create Command

```bash
curl -sS -X POST -H 'Content-Type: application/json' -d '{"apiVersion":"credentials.openchami.org/v1","kind":"BmcCredential","metadata":{"name":"test-bmc-cred-phase2"},"spec":{"targetAddress":"192.0.2.10","targetAccount":"root","secretId":"secret-1","rotationTrigger":"phase2-verification"}}' http://localhost:8080/bmccredentials/
```
