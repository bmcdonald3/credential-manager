// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// BmcCredential represents a bmccredential resource
type BmcCredential struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   fabrica.Metadata `json:"metadata"`
	Spec       BmcCredentialSpec   `json:"spec" validate:"required"`
	Status     BmcCredentialStatus `json:"status,omitempty"`
}

// BmcCredentialSpec defines the desired state of BmcCredential
type BmcCredentialSpec struct {
	Address  string `json:"address" validate:"required"`
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// BmcCredentialStatus defines the observed state of BmcCredential
type BmcCredentialStatus struct {
	Verified      bool      `json:"verified"`
	LastCheckedAt time.Time `json:"lastCheckedAt,omitempty"`
	FailureReason string    `json:"failureReason,omitempty"`
}

// Validate implements custom validation logic for BmcCredential
func (r *BmcCredential) Validate(ctx context.Context) error {
	if strings.TrimSpace(r.Spec.Address) == "" {
		return fmt.Errorf("spec.address is required")
	}

	if strings.TrimSpace(r.Spec.Username) == "" {
		return fmt.Errorf("spec.username is required")
	}

	if strings.TrimSpace(r.Spec.Password) == "" {
		return fmt.Errorf("spec.password is required")
	}

	if !isValidHostnameOrIP(r.Spec.Address) {
		return fmt.Errorf("spec.address must be a valid hostname or IP address")
	}

	return nil
}

var hostnameLabelRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

func isValidHostnameOrIP(address string) bool {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return false
	}

	if ip := net.ParseIP(trimmed); ip != nil {
		return true
	}

	if len(trimmed) > 253 || strings.HasPrefix(trimmed, ".") || strings.HasSuffix(trimmed, ".") {
		return false
	}

	labels := strings.Split(trimmed, ".")
	for _, label := range labels {
		if !hostnameLabelRe.MatchString(label) {
			return false
		}
	}

	return true
}
// GetKind returns the kind of the resource
func (r *BmcCredential) GetKind() string {
	return "BmcCredential"
}

// GetName returns the name of the resource
func (r *BmcCredential) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *BmcCredential) GetUID() string {
	return r.Metadata.UID
}

// IsHub marks this as the hub/storage version
func (r *BmcCredential) IsHub() {}
