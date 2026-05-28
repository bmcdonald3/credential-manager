# Service Specification: BMC Manager - Phase 2 (Cryptographic Integration & Execution)

## 1. System Overview

**Objective:** Integrate the local secure storage adapter to resolve encrypted credentials at runtime and execute the network action to rotate the BMC password via the Redfish API.
**Primary Domain:** Hardware Management / Out-of-band management.
**Dependencies:** Requires the previously scaffolded `BmcCredential` schema (Phase 1) and the OpenCHAMI secure storage implementation.

## 2. Package Integration (internal/secrets)

* **Action:** Create a new package at `internal/secrets`.
* **Implementation:** Add the `encryption.go` and `localStoreAdapter.go` files containing the AES-GCM encryption and HKDF key derivation logic.
* **Dependencies:** Add the required external packages to `go.mod`:
`go get golang.org/x/crypto github.com/mitchellh/mapstructure`

## 3. Server Initialization (cmd/server/main.go)

* **Configuration:** Add a mechanism to accept the `MASTER_KEY` environment variable (must be a 64-character hex string representing 32 bytes) and a path to the local secrets file (e.g., via a `--secrets-file` flag or `SECRETS_FILE` environment variable defaulting to `/tmp/vault-secrets.json`).
* **Instantiation:** Before starting the reconciliation controller, initialize the `LocalStore` using `secrets.NewLocalSecretStore(masterKey, secretsFile, true)`.
* **Dependency Injection:** Modify the reconciler registration logic (e.g., `RegisterReconcilers`) to inject the initialized `LocalStore` instance into the `BmcCredentialReconciler` struct.

## 4. Reconciliation Execution (bmccredential_reconciler.go)

* **Trigger:** The reconciler continues to intercept creation/updates of `BmcCredential`.
* **Credential Resolution:**
1. If validation from Phase 1 passes, use the injected `LocalStore` to look up the `currentPasswordSecretID` and `newPasswordSecretID`.
2. If the lookup or decryption fails for either ID, update the resource Status to "Failed" and record the cryptographic error in the failure reason. Return to prevent further execution.


* **Network Execution:**
1. Formulate an HTTP PATCH request to `https://[TargetAddress]/redfish/v1/AccountService/Accounts/[TargetAccount]`.
2. The HTTP client must have TLS certificate verification disabled (`InsecureSkipVerify: true`).
3. Apply Basic Authentication to the request using the `CurrentUsername` and the decrypted current password.
4. Set the JSON payload to `{"Password": "<decrypted_new_password>"}`.


* **State Update:**
* If the HTTP PATCH returns 200 OK or 204 No Content, update the Status state to "Success" and clear any failure reasons.
* If the request fails, returns 401 Unauthorized, or times out, update the Status state to "Failed" and record the specific HTTP status code or error message.
* Update the timestamp to the current UTC time.



## 5. Agent Operational Directives (Strict Rules of Engagement)

You are an autonomous software engineering agent. Achieve the target state defined in Sections 1-4.

**Workflow Loop & Savepoints:**

1. **Analyze & Setup:** Read the requirements. Create the `internal/secrets` package and inject the provided storage code. Run `go mod tidy`.
* *Git Action:* `git add . && git commit -m "feat: integrate secure storage adapter"`


2. **Wire Dependencies:** Modify `cmd/server/main.go` to initialize the store and pass it into the reconciliation layer.
3. **Implement Logic:** Write the decryption and HTTP Redfish rotation logic in `pkg/reconcilers/bmccredential_reconciler.go`.
4. **Verify (Compiler):** Run `go build ./...` and resolve any compilation or type assignment errors autonomously.
5. **Test (Unit):** Update or create tests in `bmccredential_reconciler_test.go` to mock the `LocalStore` and the HTTP client. Verify that decryption failures and Redfish rejections are properly recorded in the resource status. Run `go test ./...`.
* *Git Action:* `git add . && git commit -m "feat: implement credential rotation network logic"`


6. **Verify (Integration):** * Export a test master key: `export MASTER_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
* Start the server in the background using the exact required arguments (e.g., `go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"`).
* Use a script or custom command to write a mock encrypted secret to `/tmp/vault-secrets.json` matching the `MASTER_KEY` and the Secret IDs you plan to use.
* Execute a `curl` POST request to trigger the reconciler.
* Verify the logs indicate the reconciler attempted the lookup and network call.
* Terminate the background server process.


7. **Handoff:** Create a `HANDOFF-PHASE2.md` file in the root directory summarizing the injected dependencies, the structure of the HTTP request, and the verified integration test commands.