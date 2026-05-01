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
	"net/url"
	"strings"
	"time"

	v1 "github.com/user/bmc-manager/apis/example.fabrica.dev/v1"
)

const redfishPath = "/redfish/v1/"

var newHTTPClient = func() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
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
	res.Status.LastCheckedAt = time.Now().UTC()
	res.Status.Verified = false
	res.Status.FailureReason = ""

	address := strings.TrimSpace(res.Spec.Address)
	requestURL := fmt.Sprintf("https://%s%s", address, redfishPath)

	parsed, err := url.Parse(requestURL)
	if err != nil {
		res.Status.FailureReason = fmt.Sprintf("invalid verification URL: %v", err)
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		res.Status.FailureReason = fmt.Sprintf("failed to build verification request: %v", err)
		return nil
	}

	req.SetBasicAuth(res.Spec.Username, res.Spec.Password)

	resp, err := newHTTPClient().Do(req)
	if err != nil {
		res.Status.FailureReason = err.Error()
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		res.Status.Verified = true
		res.Status.FailureReason = ""
		return nil
	}

	res.Status.FailureReason = fmt.Sprintf("verification failed: received HTTP %d", resp.StatusCode)

	return nil
}
