## 1. Reconciliation Logic Summary

The service listens for changes to `BmcCredential` resources. When a resource is created or its `RotationTrigger` is updated, the reconciler constructs a Redfish PATCH request to `https://[TargetAddress]/redfish/v1/AccountService/Accounts/[TargetAccount]`. It uses the `CurrentUsername` and `CurrentPassword` for Basic Authentication and sends the `NewPassword` in the JSON payload. The HTTP response determines the success or failure recorded in the resource's Status.

## 2. Go Struct Definitions

```go
type BmcCredentialSpec struct {
    TargetAddress   string `json:"targetAddress" validate:"required"`
    TargetAccount   string `json:"targetAccount" validate:"required"`
    CurrentUsername string `json:"currentUsername" validate:"required"`
    CurrentPassword string `json:"currentPassword" validate:"required"`
    NewPassword     string `json:"newPassword" validate:"required"`
    RotationTrigger string `json:"rotationTrigger,omitempty"`
}

type BmcCredentialStatus struct {
    LastRotationAttempt *time.Time `json:"lastRotationAttempt,omitempty"`
    RotationSucceeded   bool       `json:"rotationSucceeded"`
    FailureReason       string     `json:"failureReason,omitempty"`
}

```

## 3. Server Startup Command

`go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"`

## 4. Verification Command

`curl -X POST -H "Content-Type: application/json" -d '{"apiVersion":"credentials.openchami.org/v1","kind":"BmcCredential","metadata":{"name":"test-bmc-cred"},"spec":{"targetAddress":"192.168.1.100","targetAccount":"root","currentUsername":"root","currentPassword":"oldpassword","newPassword":"newpassword","rotationTrigger":"initial-creation"}}' http://localhost:8080/bmccredentials`
