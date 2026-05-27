// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"fmt"
	"github.com/openchami/fabrica/pkg/fabrica"
)

// BmcCredential represents a bmccredential resource
type BmcCredential struct {
	APIVersion string              `json:"apiVersion"`
	Kind       string              `json:"kind"`
	Metadata   fabrica.Metadata    `json:"metadata"`
	Spec       BmcCredentialSpec   `json:"spec" validate:"required"`
	Status     BmcCredentialStatus `json:"status,omitempty"`
}

// BmcCredentialSpec defines the desired state of BmcCredential
type BmcCredentialSpec struct {
	Address         string `json:"address" validate:"required"`
	AuthUsername    string `json:"authUsername" validate:"required"`
	TargetAccount   string `json:"targetAccount" validate:"required"`
	NodeIdentifier  string `json:"nodeIdentifier" validate:"required"`
	DesiredPassword string `json:"desiredPassword,omitempty"`
}

// BmcCredentialStatus defines the observed state of BmcCredential
type BmcCredentialStatus struct {
	RotationSucceeded bool   `json:"rotationSucceeded"`
	LastRotationUTC   string `json:"lastRotationUTC,omitempty"`
	FailureReason     string `json:"failureReason,omitempty"`
}

// Validate implements custom validation logic for BmcCredential
func (r *BmcCredential) Validate(ctx context.Context) error {
	if r.Spec.Address == "" {
		return fmt.Errorf("spec.address is required")
	}
	if r.Spec.AuthUsername == "" {
		return fmt.Errorf("spec.authUsername is required")
	}
	if r.Spec.TargetAccount == "" {
		return fmt.Errorf("spec.targetAccount is required")
	}
	if r.Spec.NodeIdentifier == "" {
		return fmt.Errorf("spec.nodeIdentifier is required")
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
