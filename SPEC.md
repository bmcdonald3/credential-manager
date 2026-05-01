# Service Specification: BMC Manager

## 1. System Overview
**Objective:** Manage Baseboard Management Controller (BMC) credentials securely and verify connectivity.
**Primary Domain:** Hardware Management / Out-of-band management.
**Boundaries:** This service stores credentials and verifies they work. It does not perform power actions or firmware updates.

## 2. Infrastructure & Scaffold Configuration
This service relies on the Fabrica framework.
* **Project Name:** bmc-manager
* **API Group:** example.fabrica.dev
* **Storage Type:** ent
* **Database Driver:** sqlite
* **Required Features:** --reconcile, --events, --storage

## 3. Resource Requirements (Agent-Designed Schema)

### Resource: BmcCredential
* **Description:** Represents a set of credentials used to access a specific BMC.
* **Data to Capture (Spec):** The required information to target a specific machine over the network and authenticate using a standard username and password. Ensure the address field requires a valid hostname or IP.
* **State to Track (Status):** A boolean indicating if verification succeeded, a timestamp of the last check, and a string to hold any failure reasons.

## 4. Custom Business Logic & Reconciliation
* **Trigger:** Creation or Update of a `BmcCredential` resource.
* **Action:** The reconciler (`bmccredential_reconciler.go`) must intercept the creation/update. It must extract the target address and authentication details from the Spec. It should execute a simulated HTTP GET request to `https://[Address]/redfish/v1/` using basic authentication to verify the credentials.
* **State Update:** * If the HTTP request returns 200 OK, update the Status to reflect a verified state with no errors.
  * If the HTTP request fails or times out, update the Status to reflect an unverified state and record the error message.
  * Always update the last checked timestamp to the current UTC time.

## 5. Agent Operational Directives (Strict Rules of Engagement)
You are an autonomous software engineering agent. You must achieve the target state defined in Sections 1-4 by executing terminal commands, writing code, and resolving your own errors.

**Workflow Loop:**
1. **Analyze & Design:** Read the business logic required in Section 4. Determine the exact Go struct fields required for the Spec and Status of the resources listed in Section 3.
2. **Scaffold:** Execute the `fabrica init` command with the parameters defined in Section 2.
3. **Define:** Use `fabrica add resource` for each item in Section 3. Modify the generated `*_types.go` files to implement the schema you designed in Step 1.
4. **Generate:** Run `fabrica generate` to build the framework artifacts. 
5. **Implement:** Write the custom logic defined in Section 4 in the appropriate Fabrica reconciler stubs.
6. **Verify (CRITICAL):** You must run `go mod tidy` and `go build ./...` after modifying any Go files. If the compiler outputs errors, you must read the error, modify the code, and re-compile autonomously.
7. **Test:** Write table-driven tests for the custom reconciliation logic. Run `go test ./...`. Ensure tests pass before declaring the task complete.