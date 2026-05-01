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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/user/bmc-manager/apis/example.fabrica.dev/v1"
)

const bmcRequestTimeout = 10 * time.Second

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
	res.Status.LastRotationAttempt = time.Now().UTC()

	if err := rotateBMCPassword(ctx, res.Spec); err != nil {
		res.Status.RotationSucceeded = false
		res.Status.FailureReason = err.Error()
		res.Status.Ready = false
		res.Status.Phase = "Failed"
		res.Status.Message = err.Error()
		r.Logger.Errorf("BmcCredential rotation failed for %s: %v", res.GetUID(), err)
		return nil
	}

	res.Status.RotationSucceeded = true
	res.Status.FailureReason = ""
	res.Status.Ready = true
	res.Status.Phase = "Succeeded"
	res.Status.Message = "Credential rotation succeeded"
	r.Logger.Infof("BmcCredential rotation succeeded for %s", res.GetUID())

	return nil
}

func rotateBMCPassword(ctx context.Context, spec v1.BmcCredentialSpec) error {
	return rotateBMCPasswordWithTimeout(ctx, spec, bmcRequestTimeout)
}

func rotateBMCPasswordWithTimeout(ctx context.Context, spec v1.BmcCredentialSpec, timeout time.Duration) error {
	trimmedAddress := strings.TrimSpace(spec.TargetAddress)
	trimmedAccount := strings.TrimSpace(spec.TargetAccount)

	endpoint := fmt.Sprintf("https://%s/redfish/v1/AccountService/Accounts/%s", trimmedAddress, url.PathEscape(trimmedAccount))
	payload := map[string]string{"Password": spec.NewPassword}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal redfish payload: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPatch, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(spec.CurrentUsername, spec.CurrentPassword)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("password rotation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("password rotation unauthorized: %s", resp.Status)
	}

	return fmt.Errorf("password rotation failed with status %s", resp.Status)
}
