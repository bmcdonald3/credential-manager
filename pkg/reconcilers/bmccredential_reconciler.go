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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	v1 "github.com/user/test/apis/example.fabrica.dev/v1"
)

const bmcRequestTimeout = 10 * time.Second

// bmcHTTPClientFactory allows tests to inject a deterministic HTTP client.
var bmcHTTPClientFactory = func() *http.Client {
	return &http.Client{
		Timeout: bmcRequestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

type redfishPasswordPatch struct {
	Password string `json:"Password"`
}

func rotateBmcPassword(ctx context.Context, client *http.Client, spec v1.BmcCredentialSpec) error {
	targetAddress := strings.TrimSpace(spec.TargetAddress)
	targetAccount := strings.TrimSpace(spec.TargetAccount)
	if targetAddress == "" {
		return fmt.Errorf("target address is required")
	}
	if targetAccount == "" {
		return fmt.Errorf("target account is required")
	}

	endpoint := fmt.Sprintf("https://%s/redfish/v1/AccountService/Accounts/%s", targetAddress, url.PathEscape(targetAccount))
	payload, err := json.Marshal(redfishPasswordPatch{Password: spec.NewPassword})
	if err != nil {
		return fmt.Errorf("failed to marshal redfish password payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("failed to create redfish PATCH request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(spec.CurrentUsername, spec.CurrentPassword)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("redfish PATCH request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		return fmt.Errorf("redfish PATCH failed with status %d and unreadable body: %w", resp.StatusCode, readErr)
	}

	if len(strings.TrimSpace(string(body))) == 0 {
		return fmt.Errorf("redfish PATCH failed with status %d", resp.StatusCode)
	}

	return fmt.Errorf("redfish PATCH failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
	attemptTime := time.Now().UTC()
	res.Status.LastRotationAttempt = &attemptTime

	if err := rotateBmcPassword(ctx, bmcHTTPClientFactory(), res.Spec); err != nil {
		res.Status.RotationSucceeded = false
		res.Status.FailureReason = err.Error()
		r.Logger.Warnf("BmcCredential rotation failed for %s: %v", res.GetUID(), err)
		return nil
	}

	res.Status.RotationSucceeded = true
	res.Status.FailureReason = ""
	r.Logger.Infof("BmcCredential rotation succeeded for %s", res.GetUID())

	return nil
}
