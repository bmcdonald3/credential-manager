// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"fmt"
	"github.com/openchami/fabrica/pkg/fabrica"
	"strings"
	"time"
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
	TargetAddress    string `json:"targetAddress" validate:"required,hostname_rfc1123|ip"`
	CurrentUsername  string `json:"currentUsername" validate:"required,min=1,max=64"`
	CurrentPassword  string `json:"currentPassword" validate:"required,min=1,max=256"`
	TargetAccount    string `json:"targetAccount" validate:"required,min=1,max=128"`
	NewPassword      string `json:"newPassword" validate:"required,min=1,max=256"`
}

// BmcCredentialStatus defines the observed state of BmcCredential
type BmcCredentialStatus struct {
	RotationSucceeded bool      `json:"rotationSucceeded"`
	LastRotationAttempt time.Time `json:"lastRotationAttempt,omitempty"`
	FailureReason      string    `json:"failureReason,omitempty"`
	Phase              string    `json:"phase,omitempty"`
	Message            string    `json:"message,omitempty"`
	Ready              bool      `json:"ready"`
}

// Validate implements custom validation logic for BmcCredential
func (r *BmcCredential) Validate(ctx context.Context) error {
	if strings.TrimSpace(r.Spec.TargetAddress) == "" {
		return fmt.Errorf("spec.targetAddress is required")
	}
	if strings.TrimSpace(r.Spec.CurrentUsername) == "" {
		return fmt.Errorf("spec.currentUsername is required")
	}
	if strings.TrimSpace(r.Spec.CurrentPassword) == "" {
		return fmt.Errorf("spec.currentPassword is required")
	}
	if strings.TrimSpace(r.Spec.TargetAccount) == "" {
		return fmt.Errorf("spec.targetAccount is required")
	}
	if strings.TrimSpace(r.Spec.NewPassword) == "" {
		return fmt.Errorf("spec.newPassword is required")
	}

	return nil
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
