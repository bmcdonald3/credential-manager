
# OpenCHAMI BMC Credential Manager - Usage Guide

This document details the complete workflow for utilizing the BMC Credential Manager to rotate Baseboard Management Controller (BMC) passwords via the Redfish API. The service utilizes a secure, local JSON-backed secret store to prevent plaintext passwords from being exposed in configuration files or API payloads.

## Prerequisites

* A 64-character hexadecimal `MASTER_KEY` (32 bytes).
* Network access to the target BMC's Redfish endpoint.
* Go 1.24+ installed on the host machine.

## Step 1: Seed the Encrypted Credential Store

Before requesting a rotation, the authentication payload must be encrypted and stored. The API requires a `secretId` that references a JSON map containing `currentUsername`, `currentPassword`, and `newPassword`.

Create a file named `seed.go` to generate the `secrets.json` file. Replace the `<USERNAME>`, `<OLD_PASSWORD>`, `<NEW_PASSWORD>`, and `<ID>` parameters with your actual target data.

```go
package main

import (
    "encoding/json"
    "log"
    "os"

    "github.com/openchami/credential-manager/pkg/secrets"
)

func main() {
    masterKey := os.Getenv("MASTER_KEY")
    if masterKey == "" {
        log.Fatal("MASTER_KEY environment variable is required")
    }

    store, err := secrets.NewLocalSecretStore(masterKey, "secrets.json", true)
    if err != nil {
        log.Fatalf("Failed to initialize store: %v", err)
    }

    payload := map[string]interface{}{
        "currentUsername": "<USERNAME>",
        "currentPassword": "<OLD_PASSWORD>",
        "newPassword":     "<NEW_PASSWORD>",
    }

    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        log.Fatalf("Failed to marshal payload to JSON: %v", err)
    }

    err = store.StoreSecretByID("bmc-secret-<ID>", string(payloadBytes))
    if err != nil {
        log.Fatalf("Failed to store secret: %v", err)
    }

    log.Println("Successfully seeded bmc-secret-<ID> into secrets.json")
}

```

## Step 2: Execute the End-to-End Workflow

The following script compiles the service, starts it in the background on port 8090, executes the rotation request against the API, and verifies the result.

Create a file named `e2e_test.sh` and make it executable with `chmod +x e2e_test.sh`.

```bash
#!/bin/bash

export MASTER_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

echo "Seeding the encrypted credential store..."
go run seed.go

echo "Building the BMC Manager service..."
go build -o bmc-server ./cmd/server

echo "Starting the BMC Manager service in the background..."
./bmc-server serve --database-url="file:data.db?cache=shared&_fk=1" --port=8090 > server.log 2>&1 &
SERVER_PID=$!

echo "Waiting 5 seconds for the server to initialize..."
sleep 5

echo "Submitting the credential rotation request..."
curl -sS -X POST -H 'Content-Type: application/json' -d '{"metadata":{"name":"rotate-bmc-<ID>"},"spec":{"targetAddress":"<BMC_IP>","targetAccount":"3","secretId":"bmc-secret-<ID>","rotationTrigger":"test-run-1"}}' http://localhost:8090/bmccredentials/

echo ""
echo "Waiting 10 seconds for the reconciliation loop to execute..."
sleep 10

echo "Retrieving the final BmcCredential status..."
curl -sS http://localhost:8090/bmccredentials/

echo ""
echo "Verifying Redfish endpoint with NEW credentials..."
curl -k -s -u <USERNAME>:<NEW_PASSWORD> https://<BMC_IP>/redfish/v1/AccountService/Accounts/3

echo ""
echo "Stopping the background server..."
kill $SERVER_PID

```

Execute the script:

```bash
./e2e_test.sh

```

## Step 3: Verification

When the workflow executes successfully, querying the BMC with the new credentials returns a standard HTTP 200 Redfish payload. Querying with the old credentials returns an HTTP 401 Unauthorized equivalent.

**Verifying the new password (Success):**

```bash
# curl -k -u <USERNAME>:<NEW_PASSWORD> https://<BMC_IP>/redfish/v1/AccountService/Accounts/3
{"@odata.context":"/redfish/v1/$metadata#ManagerAccount.ManagerAccount","@odata.id":"/redfish/v1/AccountService/Accounts/3","@odata.type":"#ManagerAccount.v1_3_0.ManagerAccount","Id":"3","Name":"User Account","Description":"User Account","Enabled":true,"Password":null,"UserName":"<USERNAME>","RoleId":"Administrator","Links":{"Role":{"@odata.id":"/redfish/v1/AccountService/Roles/Administrator"}},"@odata.etag":"<ETAG_HASH>"}

```

**Verifying the old password (Failure):**

```bash
# curl -k -u <USERNAME>:<OLD_PASSWORD> https://<BMC_IP>/redfish/v1/AccountService/Accounts/3
{"error":{"code":"Base.1.5.0.GeneralError","message":"A general error has occurred. See ExtendedInfo for more information.","@Message.ExtendedInfo":[{"@odata.type":"#Message.v1_0_7.Message","MessageId":"Base.1.5.0.NoValidSession","Message":"There is no valid session established with the implementation.","Severity":"Critical","Resolution":"Establish a session before attempting any operations."},{"@odata.type":"#Message.v1_0_7.Message","MessageId":"Base.1.5.0.ResourceAtUriUnauthorized","Message":"While accessing the resource at /redfish/v1/AccountService/Accounts/3, the service received an authorization error failed.","MessageArgs":["/redfish/v1/AccountService/Accounts/3","failed"],"Severity":"Critical","Resolution":"Ensure that the appropriate access is provided for the service in order for it to access the URI."}]}}

```
