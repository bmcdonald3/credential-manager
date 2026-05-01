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
	"fmt"
	"net/http"
	"time"

	v1 "github.com/user/bmc-manager/apis/example.fabrica.dev/v1"
)

var (
	defaultBMCVerifyClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	nowUTC = func() time.Time {
		return time.Now().UTC()
	}
)

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
	res.Status.LastCheckedAt = nowUTC()

	err := verifyRedfishCredential(ctx, defaultBMCVerifyClient, res.Spec.Address, res.Spec.Username, res.Spec.Password)
	if err != nil {
		res.Status.Verified = false
		res.Status.FailureReason = err.Error()
		res.Status.Ready = false
		res.Status.Phase = "Unverified"
		res.Status.Message = "Credential verification failed"
		r.Logger.Warnf("BMC credential verification failed for %s: %v", res.GetUID(), err)
		return nil
	}

	res.Status.Verified = true
	res.Status.FailureReason = ""
	res.Status.Ready = true
	res.Status.Phase = "Verified"
	res.Status.Message = "Credential verification succeeded"
	r.Logger.Infof("BMC credential verified for %s", res.GetUID())

	return nil
}

func verifyRedfishCredential(ctx context.Context, client *http.Client, address, username, password string) error {
	url := fmt.Sprintf("https://%s/redfish/v1/", address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create verification request: %w", err)
	}
	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("verification request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("verification returned status %d", resp.StatusCode)
	}

	return nil
}
