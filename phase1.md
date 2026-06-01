# Service Specification: credential-manager (Phase 1)

## 1. System Overview & Architecture

**Objective:** A service that manages hardware credentials by executing secure, remote password rotation against Redfish-compatible BMC endpoints.
**Primary Domain:** Hardware lifecycle and access management.
**Boundaries:** This service should not manage non-hardware credentials (e.g., user SSO, database passwords) and, in Phase 1, does not implement localized encryption of the credentials at rest.

**Fabrica Configuration:**

* **Project Name:** credential-manager
* **API Group:** credentials.openchami.org
* **Storage Type:** ent
* **Database Driver:** sqlite
* **Required Features:** --reconcile, --events

## 2. Business Logic & Resource Schema

### Resource: BmcCredential

* **Trigger:** The reconciliation logic executes upon the initial creation of a `BmcCredential` resource, or whenever the `RotationTrigger` string is modified.
* **Reconciliation Action:** Connect to the BMC over HTTPS (skipping TLS verification), authenticate using the provided `CurrentUsername` and `CurrentPassword` via Basic Auth, and execute a Redfish PATCH request to the AccountService to update the password to `NewPassword`.
* **Required Input (Spec):**
* `TargetAddress` (string, required): The IP address or hostname of the BMC.
* `TargetAccount` (string, required): The Redfish account name to rotate.
* `CurrentUsername` (string, required): The username used to authorize the PATCH request.
* `CurrentPassword` (string, required): The password used to authorize the PATCH request.
* `NewPassword` (string, required): The new password to be set.
* `RotationTrigger` (string, optional): An arbitrary string (such as a timestamp or UUID). Updating this field signals the reconciler to execute the rotation again.


* **Required State (Status):**
* `LastRotationAttempt` (pointer to time.Time): The exact time the service attempted to connect and patch the BMC.
* `RotationSucceeded` (bool): Indicates if the HTTP request returned a 200/204 status code.
* `FailureReason` (string, optional): The error message or HTTP response body if the rotation failed.



## 3. Execution & Acceptance Criteria

Execute the framework scaffolding, resource generation, and code implementation to fulfill Sections 1 and 2. You must achieve the following criteria to complete this task:

* **Compilation:** All Go files must compile without errors. Run `go mod tidy` and `go build ./...` after any code modifications. Resolve any compiler errors autonomously.
* **Testing:** Table-driven tests for the custom reconciliation logic must be written and pass via `go test ./...`.
* **Runtime Verification:** The server must successfully bind to the port and route HTTP requests. Verify this locally by starting the server in the background using the required arguments.
* **Endpoint Validation:** You must execute a `curl` POST request to the local endpoint to create the generated resource and receive a successful 2xx HTTP status code. If a 4xx or 5xx code is returned, analyze the logs, correct the implementation, and re-test.

## 4. Output Artifacts

Upon meeting all Acceptance Criteria, generate a `HANDOFF.md` file in the root directory containing the following content:

### 1. Reconciliation Logic Summary

The service listens for changes to `BmcCredential` resources. When a resource is created or its `RotationTrigger` is updated, the reconciler constructs a Redfish PATCH request to `https://[TargetAddress]/redfish/v1/AccountService/Accounts/[TargetAccount]`. It uses the `CurrentUsername` and `CurrentPassword` for Basic Authentication and sends the `NewPassword` in the JSON payload. The HTTP response determines the success or failure recorded in the resource's Status.

### 2. Go Struct Definitions

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

### 3. Server Startup Command

`go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"`

### 4. Verification Command

`curl -X POST -H "Content-Type: application/json" -d '{"apiVersion":"credentials.openchami.org/v1","kind":"BmcCredential","metadata":{"name":"test-bmc-cred"},"spec":{"targetAddress":"192.168.1.100","targetAccount":"root","currentUsername":"root","currentPassword":"oldpassword","newPassword":"newpassword","rotationTrigger":"initial-creation"}}' http://localhost:8080/apis/credentials.openchami.org/v1/bmccredentials`