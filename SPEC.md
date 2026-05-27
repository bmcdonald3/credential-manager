# Service Specification: BMC Manager

## 1. System Overview
**Objective:** Manage and rotate Baseboard Management Controller (BMC) credentials securely over the network.
**Primary Domain:** Hardware Management / Out-of-band management.
**Boundaries:** This service updates credentials on the target device. It does not perform power actions or firmware updates.

## 2. Infrastructure & Scaffold Configuration
This service relies on the Fabrica framework.
* **Project Name:** bmc-manager
* **API Group:** example.fabrica.dev
* **Storage Type:** ent
* **Database Driver:** sqlite
* **Required Features:** --reconcile, --events, --storage

## 3. Architecture & Interfaces (Adapter Pattern)
Before implementing resource reconciliation, establish a generic abstraction layer for secret management to avoid hardcoding a specific backend.
* **Interface Definition:** Create a `SecretStore` interface in a dedicated package (e.g., `pkg/secrets`).
* **Required Methods:** `Read(id string) (string, error)`, `Write(id string, data string) error`, `Update(id string, data string) error`, `Delete(id string) error`.
* **Fallback Implementation:** Implement a `LocalSecretStore` struct that fulfills this interface. For the initial scaffold, this can utilize a simple temporary in-memory map or a standard local file implementation.

## 4. Resource Requirements (Agent-Designed Schema)

### Resource: BmcCredential
* **Description:** Represents a request to rotate credentials on a specific BMC.
* **Data to Capture (Spec):** * Target Address (hostname or IP).
  * Target Account ID or Username to update.
  * Node Identifier (e.g., UUID or MAC address). This will serve as the unique `id` passed to the `SecretStore` interface.
* **State to Track (Status):** A boolean indicating if the rotation succeeded, a timestamp of the last rotation attempt, and a string to hold any failure reasons.

## 5. Custom Business Logic & Reconciliation
* **Trigger:** Creation or Update of a `BmcCredential` resource.
* **Action:** The reconciler (`bmccredential_reconciler.go`) must intercept the creation/update and utilize the `SecretStore` interface injected into it.
  1. Extract the target address, target account ID, and node identifier from the Spec.
  2. Call `SecretStore.Read(NodeIdentifier)` to retrieve the *current* password for the target BMC.
  3. Generate a new cryptographically secure random password.
  4. Execute an HTTP PATCH request to `https://[Address]/redfish/v1/AccountService/Accounts/[TargetAccount]` using basic authentication with the *current* credentials.
  5. The JSON payload for the PATCH request must set the `Password` field to the *new* password.
  6. Ensure the HTTP client disables TLS certificate verification (`InsecureSkipVerify: true`) as BMCs typically use self-signed certificates.
  7. Upon a successful HTTP response, call `SecretStore.Update(NodeIdentifier, NewPassword)` to persist the updated credential to the backend.
* **State Update:** * If the HTTP PATCH returns 200 OK or 204 No Content, update the Status to reflect a successful rotation with no errors.
  * If the HTTP request fails, returns 401 Unauthorized, or times out, update the Status to reflect a failed rotation and record the exact error message.
  * Always update the last rotation timestamp to the current UTC time.

## 6. Agent Operational Directives (Strict Rules of Engagement)
You are an autonomous software engineering agent. You must achieve the target state defined in Sections 1-5 by executing terminal commands, writing code, and resolving your own errors.

**Workflow Loop & Savepoints:**
1. **Analyze & Design:** Read the business logic required in Section 5. Determine the exact Go struct fields required for the Spec and Status of the resources listed in Section 4.
2. **Scaffold:** Execute the `fabrica init` command with the parameters defined in Section 2. 
    * *Git Action:* `git add . && git commit -m "chore: scaffold project"`
3. **Define Interfaces:** Create the package and interfaces defined in Section 3.
    * *Git Action:* `git add . && git commit -m "feat: implement secret store interface and fallback"`
4. **Define & Generate:** Use `fabrica add resource` for the item in Section 4. Modify the generated `*_types.go` files to implement the schema you designed. Run `fabrica generate`.
    * *Git Action:* `git add . && git commit -m "feat: define resources and generate artifacts"`
5. **Implement:** Write the custom logic defined in Section 5 in the appropriate Fabrica reconciler stubs. Inject the `SecretStore` into the reconciler.
6. **Verify (Compiler):** You must run `go mod tidy` and `go build ./...` after modifying any Go files. If the compiler outputs errors, you must read the error, modify the code, and re-compile autonomously.
7. **Test (Unit):** Write table-driven tests for the custom reconciliation logic. Run `go test ./...`. Ensure tests pass.
    * *Git Action:* `git add . && git commit -m "feat: implement and test reconciliation logic"`
8. **Verify (Integration):** You must verify the server successfully binds to the port and routes HTTP requests.
    * Start the server locally in the background using the exact required arguments (e.g., `go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"`).
    * Execute a `curl` POST request to the local endpoint to create the generated resource.
    * If the response is a 404, 400, or 500, analyze the server logs, correct the payload or endpoint path, and re-test until you receive a successful 2xx HTTP status code.
    * Terminate the background server process.
9. **Handoff (CRITICAL):** Create a `HANDOFF.md` file in the root directory. This file must contain:
    * A brief summary of the business logic implemented.
    * The exact schema fields decided upon for the Spec and Status.
    * The exact, verified `curl` command that succeeded in Step 8.
    * The exact, verified server startup command used in Step 8.