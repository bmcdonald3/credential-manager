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
	"strings"
	"time"

	v1 "github.com/openchami/credential-manager/apis/credentials.openchami.org/v1"
)

var buildRedfishHTTPClient = func() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Required for Phase 1
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
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
	res.Status.FailureReason = failureReason

	if succeeded {
		res.Status.ObservedTriggerValue = res.Spec.RotationTrigger
		r.Logger.Infof("Password rotation succeeded for BmcCredential %s", res.GetUID())
		return nil
	}

	if updateErr := r.UpdateStatus(ctx, res); updateErr != nil {
		return fmt.Errorf("rotation failed (%v) and status update failed: %w", err, updateErr)
	}

	if err == nil {
		err = errors.New(failureReason)
	}

	r.Logger.Errorf("Password rotation failed for BmcCredential %s: %v", res.GetUID(), err)

	return err
}

func shouldRotate(res *v1.BmcCredential) bool {
	if res.Status.LastRotationAttempt == nil {
		return true
	}

	return res.Status.ObservedTriggerValue != res.Spec.RotationTrigger
}

func rotateRedfishPassword(ctx context.Context, spec v1.BmcCredentialSpec) (bool, string, error) {
	endpoint := fmt.Sprintf("https://%s/redfish/v1/AccountService/Accounts/%s", spec.TargetAddress, url.PathEscape(spec.TargetAccount))

	payload := map[string]string{"Password": spec.NewPassword}
	body, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Sprintf("failed to encode request body: %v", err), err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return false, fmt.Sprintf("failed to build request: %v", err), err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(spec.CurrentUsername, spec.CurrentPassword)

	fmt.Printf("\n[DEBUG] Executing Redfish payload via equivalent curl:\n"+
		"curl -k -u \"%s:%s\" -X PATCH %s -H \"Content-Type: application/json\" -d '%s'\n\n",
		spec.CurrentUsername, spec.CurrentPassword, endpoint, string(body))

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
	return false, failureReason, errors.New(failureReason)
}
