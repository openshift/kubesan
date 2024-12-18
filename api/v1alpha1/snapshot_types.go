// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type SnapshotSpec struct {
	// Should be set from creation and never updated.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	VgName string `json:"vgName"`

	// Should be set from creation and never updated.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	SourceVolume string `json:"sourceVolume"`
}

type SnapshotStatus struct {
	// The generation of the spec used to produce this status.  Useful
	// as a witness when waiting for status to change.
	ObservedGeneration int64 `json:"observedGeneration"`

	// Conditions
	// Available: The snapshot can be sourced by volumes.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []conditionsv1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// The size of the snapshot, immutable once set.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	SizeBytes *int64 `json:"sizeBytes"`

	// The file system type of the snapshot. `nil` if a snapshot of a block volume.
	// +kubebuilder:validation:XValidation:rule=oldSelf==self
	FsType *string `json:"fsType,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=snap;snaps,categories=kubesan;lv
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="VG",type=string,JSONPath=`.spec.vgName`,description='VG owning the snapshot'
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.sourceVolume`,description='Volume that this snapshot was created on'
// +kubebuilder:printcolumn:name="Available",type=date,JSONPath=`.status.conditions[?(@.type=="Available")].lastTransitionTime`,description='Time since snapshot was available'
// +kubebuilder:printcolumn:name="Size",type=integer,JSONPath=`.status.sizeBytes`,description='Size of snapshot'
// +kubebuilder:printcolumn:name="FSType",type=string,JSONPath=`.status.fsType`,description='filesystem type (blank if block)',priority=1

type Snapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotSpec   `json:"spec,omitempty"`
	Status SnapshotStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Snapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Snapshot{}, &SnapshotList{})
}
