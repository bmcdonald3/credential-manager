# BMC Manager

## Overview
The BMC Manager is a service built on the Fabrica framework designed to manage and rotate Baseboard Management Controller (BMC) credentials securely over the network. It operates using a Kubernetes-style reconciliation loop, where the desired state (a credential rotation request) is submitted to the API, and the service asynchronously attempts to apply that state to the target hardware via the Redfish API.

**Boundaries:** This service strictly handles credential rotation via HTTP PATCH requests to `/redfish/v1/AccountService/Accounts/{Id}`. It does not handle power states, firmware updates, or other BMC management functions.

## Usage

### 1. Start the Server
Run the generated Fabrica server with the SQLite database driver configured. 

```bash
go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"
```

### 2. Submit a Rotation Request
Submit a `BmcCredential` resource to the REST API. The reconciler will intercept the creation event and attempt to rotate the password using the `currentUsername` and `currentPassword` to authenticate the request.

```bash
curl -X POST http://127.0.0.1:8090/bmccredentials -H "Content-Type: application/json" -d '{"apiVersion":"v1","kind":"BmcCredential","metadata":{"name":"rotate-admin"},"spec":{"targetAddress":"172.24.0.3","currentUsername":"root","currentPassword":"initial0","targetAccount":"3","newPassword":"new-pass"}}'
```

### 3. Check the Status
Retrieve the resource to view the reconciliation outcome.

```bash
curl -X GET http://127.0.0.1:8090/bmccredentials
```

## Example Outcomes

The service updates the `status` block of the `BmcCredential` resource based on the response from the target BMC. Below are examples of observed states.

### Successful Rotation
Occurs when the Redfish API accepts the PATCH request (typically returning a 200 OK or 204 No Content).

```json
{
  "spec": {
    "targetAddress": "172.24.0.3",
    "currentUsername": "root",
    "currentPassword": "initial0",
    "targetAccount": "3",
    "newPassword": "new-pass"
  },
  "status": {
    "rotationSucceeded": true,
    "lastRotationAttempt": "2026-05-01T18:13:32.404002547Z"
  }
}
```

### Authentication Failure (401 Unauthorized)
Occurs when the `currentUsername` or `currentPassword` provided in the Spec are invalid for the target BMC.

```json
{
  "spec": {
    "targetAddress": "172.24.0.3",
    "currentUsername": "root",
    "currentPassword": "root",
    "targetAccount": "3",
    "newPassword": "initial0"
  },
  "status": {
    "rotationSucceeded": false,
    "lastRotationAttempt": "2026-05-01T18:17:12.322018266Z",
    "failureReason": "redfish PATCH failed with status 401: {\"error\":{\"code\":\"Base.1.5.0.GeneralError\",\"message\":\"A general error has occurred...\"}}"
  }
}
```

### Internal BMC Error (500 Server Error)
Occurs when the BMC rejects the new password (e.g., due to complexity requirements or reuse history) or experiences an internal fault.

```json
{
  "spec": {
    "targetAddress": "172.24.0.3",
    "currentUsername": "root",
    "currentPassword": "new-pass",
    "targetAccount": "3",
    "newPassword": "initial0"
  },
  "status": {
    "rotationSucceeded": false,
    "lastRotationAttempt": "2026-05-01T18:15:28.799676853Z",
    "failureReason": "redfish PATCH failed with status 500: {\"error\":{\"code\":\"Base.1.5.0.GeneralError\",\"message\":\"...Property Password update failed...\"}}"
  }
}
```

### Network Timeout
Occurs when the target address is unreachable or drops packets. The reconciler enforces a 10-second timeout on the HTTP client.

```json
{
  "spec": {
    "targetAddress": "192.0.2.10",
    "currentUsername": "admin",
    "currentPassword": "old-pass",
    "targetAccount": "2",
    "newPassword": "new-pass"
  },
  "status": {
    "rotationSucceeded": false,
    "lastRotationAttempt": "2026-05-01T18:12:35.327617728Z",
    "failureReason": "redfish PATCH request failed: Patch \"https://192.0.2.10/redfish/v1/AccountService/Accounts/2\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
  }
}
```
