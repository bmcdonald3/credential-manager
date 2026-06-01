// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
// This file contains user-customizable reconciliation logic for BmcCredential.
//
// ⚠️ This file is safe to edit - it will NOT be overwritten by code generation.
package reconcilers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	v1 "github.com/openchami/credential-manager/apis/credentials.openchami.org/v1"
	"github.com/openchami/credential-manager/pkg/secrets"
)

const (
	masterKeyEnvVar = "MASTER_KEY"
	secretStoreFile = "secrets.json"
)

type resolvedCredentials struct {
	CurrentUsername string
	CurrentPassword string
	NewPassword     string
}

type secretPayload struct {
	CurrentUsername string `json:"currentUsername"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	Username        string `json:"username"`
	Password        string `json:"password"`
}

type terminalError struct {
	reason string
}

func (e *terminalError) Error() string {
	return e.reason
}

func newTerminalError(format string, args ...interface{}) error {
	return &terminalError{reason: fmt.Sprintf(format, args...)}
}

func isTerminalError(err error) bool {
	var target *terminalError
	return errors.As(err, &target)
}

var buildRedfishHTTPClient = func() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Required for Phase 1
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
}

var newSecretStore = func(masterKeyHex, filename string, create bool) (secrets.SecretStore, error) {
	return secrets.NewLocalSecretStore(masterKeyHex, filename, create)
}

// reconcileBmcCredential contains custom reconciliation logic.
//
// This method is called by the generated Reconcile() orchestration method.
// Implement BmcCredential-specific reconciliation logic here.
//
// Guidelines:
//  1. Keep this method idempotent (safe to call multiple times)
//  2. Update Status fields to reflect observed state
//  3. Emit events for significant state changes using r.EmitEvent()
//  4. Use r.Logger for debugging (Infof, Warnf, Errorf, Debugf)
//  5. Return errors for transient failures (will retry with backoff)
//  6. Access storage via r.Client (Get, List, Update, Create, Delete)
//
// Example implementation patterns:
//
// For hardware resources (BMC, Node):
//   - Connect to hardware endpoint
//   - Query current state
//   - Update Status.Connected, Status.Version, Status.Health
//   - Emit events when state changes
//
// For hierarchical resources (Rack, Chassis):
//   - Create/reconcile child resources
//   - Update Status with child counts and references
//   - Emit events when topology changes
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - res: The BmcCredential resource to reconcile
//
// Returns:
//   - error: If reconciliation failed (will trigger retry with backoff)
func (r *BmcCredentialReconciler) reconcileBmcCredential(ctx context.Context, res *v1.BmcCredential) error {
	if !shouldRotate(res) {
		r.Logger.Debugf("Skipping rotation for %s because trigger is unchanged", res.GetUID())
		return nil
	}

	attempt := time.Now().UTC()
	res.Status.LastRotationAttempt = &attempt

	succeeded, failureReason, err := rotateRedfishPassword(ctx, res.Spec)
	res.Status.RotationSucceeded = succeeded
	if succeeded {
		res.Status.FailureReason = ""
	} else {
		res.Status.FailureReason = appendFailureReason(res.Status.FailureReason, failureReason)
	}

	if succeeded {
		res.Status.ObservedTriggerValue = res.Spec.RotationTrigger
		r.Logger.Infof("Password rotation succeeded for BmcCredential %s", res.GetUID())
		return nil
	}

	if err == nil {
		err = errors.New(failureReason)
	}

	if isTerminalError(err) {
		res.Status.ObservedTriggerValue = res.Spec.RotationTrigger
		r.Logger.Errorf("Terminal password rotation failure for BmcCredential %s: %v", res.GetUID(), err)
		return nil
	}

	r.Logger.Errorf("Transient password rotation failure for BmcCredential %s: %v", res.GetUID(), err)

	return err
}

func shouldRotate(res *v1.BmcCredential) bool {
	if res.Status.LastRotationAttempt == nil {
		return true
	}

	return res.Status.ObservedTriggerValue != res.Spec.RotationTrigger
}

func rotateRedfishPassword(ctx context.Context, spec v1.BmcCredentialSpec) (bool, string, error) {
	if strings.TrimSpace(spec.TargetAddress) == "" {
		return false, "missing required spec.targetAddress", newTerminalError("missing required spec.targetAddress")
	}
	if strings.TrimSpace(spec.TargetAccount) == "" {
		return false, "missing required spec.targetAccount", newTerminalError("missing required spec.targetAccount")
	}
	if strings.TrimSpace(spec.SecretID) == "" {
		return false, "missing required spec.secretId", newTerminalError("missing required spec.secretId")
	}

	masterKey := strings.TrimSpace(os.Getenv(masterKeyEnvVar))
	if masterKey == "" {
		return false, "MASTER_KEY environment variable is required", newTerminalError("MASTER_KEY environment variable is required")
	}

	store, err := newSecretStore(masterKey, secretStoreFile, false)
	if err != nil {
		return false, fmt.Sprintf("failed to open secret store: %v", err), newTerminalError("failed to open secret store: %v", err)
	}

	secretJSON, err := store.GetSecretByID(spec.SecretID)
	if err != nil {
		return false, fmt.Sprintf("failed to load secret %q: %v", spec.SecretID, err), newTerminalError("failed to load secret %q: %v", spec.SecretID, err)
	}

	credentials, err := decodeSecretPayload(secretJSON)
	if err != nil {
		return false, err.Error(), newTerminalError("%s", err.Error())
	}

	endpoint := fmt.Sprintf("https://%s/redfish/v1/AccountService/Accounts/%s", spec.TargetAddress, url.PathEscape(spec.TargetAccount))

	payload := map[string]string{"Password": credentials.NewPassword}
	body, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Sprintf("failed to encode request body: %v", err), err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return false, fmt.Sprintf("failed to build request: %v", err), err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(credentials.CurrentUsername, credentials.CurrentPassword)

	resp, err := buildRedfishHTTPClient().Do(req)
	if err != nil {
		return false, fmt.Sprintf("request to BMC failed: %v", err), err
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		return false, fmt.Sprintf("failed to read response body: %v", readErr), readErr
	}

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return true, "", nil
	}

	trimmed := strings.TrimSpace(string(responseBody))
	if trimmed == "" {
		trimmed = "no response body"
	}

	failureReason := fmt.Sprintf("received HTTP %d from BMC: %s", resp.StatusCode, trimmed)
	if resp.StatusCode >= http.StatusInternalServerError {
		return false, failureReason, errors.New(failureReason)
	}

	return false, failureReason, newTerminalError("%s", failureReason)
}

func decodeSecretPayload(secretJSON string) (resolvedCredentials, error) {
	var payload secretPayload
	if err := json.Unmarshal([]byte(secretJSON), &payload); err != nil {
		return resolvedCredentials{}, fmt.Errorf("invalid secret payload JSON: %w", err)
	}

	credentials := resolvedCredentials{
		CurrentUsername: strings.TrimSpace(firstNonEmpty(payload.CurrentUsername, payload.Username)),
		CurrentPassword: strings.TrimSpace(firstNonEmpty(payload.CurrentPassword, payload.Password)),
		NewPassword:     strings.TrimSpace(payload.NewPassword),
	}

	if credentials.CurrentUsername == "" || credentials.CurrentPassword == "" || credentials.NewPassword == "" {
		return resolvedCredentials{}, errors.New("invalid secret payload JSON: expected currentUsername/currentPassword/newPassword fields")
	}

	return credentials, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func appendFailureReason(existing, next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return strings.TrimSpace(existing)
	}

	existing = strings.TrimSpace(existing)
	if existing == "" {
		return next
	}

	return existing + "; " + next
}
