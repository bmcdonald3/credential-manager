## 5. Agent Operational Directives (Strict Rules of Engagement)
You are an autonomous software engineering agent. You must achieve the target state defined in Sections 1-4 by executing terminal commands, writing code, and resolving your own errors.

**Workflow Loop & Savepoints:**
1. **Analyze & Design:** Read the business logic required in Section 4. Determine the exact Go struct fields required for the Spec and Status of the resources listed in Section 3.
2. **Scaffold:** Execute the `fabrica init` command with the parameters defined in Section 2. 
    * *Git Action:* `git add . && git commit -m "chore: scaffold project"`
3. **Define & Generate:** Use `fabrica add resource` for each item in Section 3. Modify the generated `*_types.go` files to implement the schema you designed. Run `fabrica generate`.
    * *Git Action:* `git add . && git commit -m "feat: define resources and generate artifacts"`
4. **Implement:** Write the custom logic defined in Section 4 in the appropriate Fabrica reconciler stubs.
5. **Verify (Compiler):** You must run `go mod tidy` and `go build ./...` after modifying any Go files. If the compiler outputs errors, you must read the error, modify the code, and re-compile autonomously.
6. **Test (Unit):** Write table-driven tests for the custom reconciliation logic. Run `go test ./...`. Ensure tests pass.
    * *Git Action:* `git add . && git commit -m "feat: implement and test reconciliation logic"`
7. **Verify (Integration):** You must verify the server successfully binds to the port and routes HTTP requests.
    * Start the server locally in the background using the exact required arguments (e.g., `go run ./cmd/server serve --database-url="file:data.db?cache=shared&_fk=1"`).
    * Execute a `curl` POST request to the local endpoint to create the generated resource.
    * If the response is a 404, 400, or 500, analyze the server logs, correct the payload or endpoint path, and re-test until you receive a successful 2xx HTTP status code.
    * Terminate the background server process.
8. **Handoff (CRITICAL):** Create a `HANDOFF.md` file in the root directory. This file must contain:
    * A brief summary of the business logic implemented.
    * The exact schema fields decided upon for the Spec and Status.
    * The exact, verified `curl` command that succeeded in Step 7.
    * The exact, verified server startup command used in Step 7.