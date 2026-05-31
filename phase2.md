# Phase 2: Reconciliation Implementation - credential-manager

## 1. Context Acquisition

Read the `HANDOFF.md` file in the root directory to understand the existing schema for `Spec` and `Status`. Do not modify the underlying database driver or storage types.

Refactor the `BmcCredentialSpec` generated in Phase 1 to remove the `CurrentUsername`, `CurrentPassword`, and `NewPassword` strings. Replace them with a single `SecretID` (string, required) field.

You will utilize the Magellan secret handling code that has been manually copied into the `pkg/secrets` directory. The secret store conforms to the following interface:

```go
type SecretStore interface {
	GetSecretByID(secretID string) (string, error)
	StoreSecretByID(secretID, secret string) error
	ListSecrets() (map[string]string, error)
	RemoveSecretByID(secretID string) error
}

```

The constructor signature is:

```go
func NewLocalSecretStore(masterKeyHex, filename string, create bool) (*LocalSecretStore, error)

```

## 2. Reconciliation State Machine

Implement the following logic inside the generated Fabrica reconciler loop.

* **Pre-flight Checks:**
* Validate that the `MASTER_KEY` environment variable is present in the server environment.
* Verify that `TargetAddress`, `TargetAccount`, and `SecretID` are present in the Spec.


* **Execution Steps:**
* Step 1: Instantiate the copied Magellan `LocalSecretStore` using the `MASTER_KEY` environment variable and the local `secrets.json` file via `secrets.NewLocalSecretStore`.
* Step 2: Call `GetSecretByID(SecretID)` to decrypt the credential payload from the local JSON file.
* Step 3: Unmarshal the decrypted JSON string to extract the `CurrentUsername`, `CurrentPassword`, and `NewPassword` values.
* Step 4: Connect to the BMC over HTTPS (skipping TLS verification), use the extracted `CurrentUsername` and `CurrentPassword` for Basic Authentication, and execute the Redfish PATCH request to update the account to the `NewPassword`.


* **Error Handling:**
* Terminal errors: Missing `MASTER_KEY`, missing `secrets.json` file, `SecretID` not found in the store, or invalid JSON payload format within the decrypted secret. These indicate unrecoverable misconfigurations. Do not return the error to the controller (which would trigger an infinite retry).
* Transient errors: Network timeouts reaching the BMC, TCP connection resets, or temporary 5xx HTTP responses from the BMC. Return these errors to the controller.
* Retry strategy: Rely on the default Fabrica exponential backoff queue by returning transient errors up the stack.



## 3. State Updates

Based on the execution steps, update the resource's `Status` field explicitly.

* **On Success:** Set `Status.RotationSucceeded = true`, clear `Status.FailureReason`, and set `Status.LastRotationAttempt` to the current UTC time.
* **On Transient Failure:** Set `Status.RotationSucceeded = false`, append the network/BMC error message to `Status.FailureReason`, and set `Status.LastRotationAttempt` to the current UTC time.
* **On Terminal Failure:** Set `Status.RotationSucceeded = false`, append the secret lookup/decryption error message to `Status.FailureReason`, set `Status.LastRotationAttempt` to the current UTC time, and halt further reconciliation by returning `nil`.

## 4. Acceptance Criteria

* **Compilation:** The code must compile. Run `go mod tidy` and `go build ./...`.
* **Testing:** Write specific unit tests targeting the new error handling and state transitions in the reconciler. Run `go test ./...`. Ensure tests mock the `SecretStore` interface to simulate decryption failures and successful payload extraction without relying on a real `secrets.json` file.
* **Idempotency Verification:** The reconciler must be idempotent. It should be able to run multiple times against the same `Spec` without duplicating external resources or throwing state errors.

## 5. Output Artifacts

Upon meeting all Acceptance Criteria, generate a `HANDOFF-PHASE2.md` file in the root directory containing:

1. A brief summary of the implemented reconciliation logic.
2. The final Go struct definitions for the Spec and Status.
3. The exact, verified server startup command used during runtime verification.
4. The exact, verified `curl` command that successfully created the resource.