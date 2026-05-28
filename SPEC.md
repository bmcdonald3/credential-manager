# Service Specification: BMC Manager - Phase 1 (Schema & Scaffolding)

## 1. System Overview

**Objective:** Establish the foundational API schema and service scaffolding for the Baseboard Management Controller (BMC) credential manager.
**Primary Domain:** Hardware Management / Out-of-band management.
**Boundaries:** This phase is strictly limited to establishing the data model and API endpoints. It does not implement secret decryption or external network actions against the BMC hardware.

## 2. Infrastructure & Scaffold Configuration

This service relies on the Fabrica framework.

* **Project Name:** bmc-manager
* **API Group:** example.fabrica.dev
* **Storage Type:** ent
* **Database Driver:** sqlite
* **Required Features:** --reconcile, --events, --storage

## 3. Resource Requirements (Agent-Designed Schema)

### Resource: BmcCredential

* **Description:** Represents a request to rotate credentials on a specific BMC.
* **Data to Capture (Spec):** * Target Address (hostname or IP).
* Current Username (required for initial authentication).
* Current Password Secret ID (reference to the encrypted current password; DO NOT store plaintext).
* Target Account ID or Username to update.
* New Password Secret ID (reference to the encrypted new password; DO NOT store plaintext).


* **State to Track (Status):** A string indicating the reconciliation state (e.g., "Pending", "Validated", "Failed"), a timestamp of the last update, and a string to hold any validation failure reasons.

## 4. Custom Business Logic & Reconciliation

* **Trigger:** Creation or Update of a `BmcCredential` resource.
* **Action:** The reconciler (`bmccredential_reconciler.go`) must intercept the creation/update.
1. Extract the Spec fields.
2. Validate that `CurrentPasswordSecretID` and `NewPasswordSecretID` are not empty strings.
3. This phase does NOT implement the Redfish HTTP PATCH logic or the LocalStore lookup.


* **State Update:** * If validation passes, update the Status to "Validated" and clear any failure reasons.
* If validation fails (e.g., missing Secret IDs), update the Status to "Failed" and record the validation error message.
* Update the timestamp to the current UTC time.



## 5. Agent Operational Directives (Strict Rules of Engagement)

You are an autonomous software engineering agent. You must achieve the target state defined in Sections 1-4 by executing terminal commands, writing code, and resolving your own errors.

**Workflow Loop & Savepoints:**

1. **Analyze & Design:** Read the business logic required in Section 4. Determine the exact Go struct fields required for the Spec and Status of the resources listed in Section 3.
2. **Scaffold:** Execute the `fabrica init` command with the parameters defined in Section 2.
* *Git Action:* `git add . && git commit -m "chore: scaffold project"`


3. **Define & Generate:** Use `fabrica add resource` for the item in Section 3. Modify the generated `*_types.go` files to implement the schema you designed. Run `fabrica generate`.
* *Git Action:* `git add . && git commit -m "feat: define resources and generate artifacts"`


4. **Implement:** Write the custom validation logic defined in Section 4 in the appropriate Fabrica reconciler stubs.
5. **Verify (Compiler):** You must run `go mod tidy` and `go build ./...` after modifying any Go files. If the compiler outputs errors, you must read the error, modify the code, and re-compile autonomously.
6. **Test (Unit):** Write table-driven tests for the custom reconciliation validation logic. Run `go test ./...`. Ensure tests pass.
* *Git Action:* `git add . && git commit -m "feat: implement and test reconciliation validation logic"`


7. **Verify (Integration):** You must verify the server successfully binds to the port and routes HTTP requests.
* Start the server locally in the background using the exact required arguments (e.g., `go run ./cmd/server/main.go serve --database-url="file:data.db?cache=shared&_fk=1"`).
* Execute a `curl` POST request to the local endpoint to create the generated resource utilizing the new Secret ID fields.
* If the response is a 404, 400, or 500, analyze the server logs, correct the payload or endpoint path, and re-test until you receive a successful 2xx HTTP status code.
* Terminate the background server process.


8. **Handoff (CRITICAL):** Create a `HANDOFF.md` file in the root directory. This file must contain:
* A brief summary of the schema and validation logic implemented.
* The exact schema fields decided upon for the Spec and Status.
* The exact, verified `curl` command that succeeded in Step 7.
* The exact, verified server startup command used in Step 7.