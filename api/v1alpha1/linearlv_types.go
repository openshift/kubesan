// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type LinearLvSpec struct {
	// Should be set from creation and never updated.
	VgName string `json:"vgName"`

	// Should be set from creation and never updated.
	ReadOnly bool `json:"readOnly"`

	// Must be positive and a multiple of 512. May be updated at will, but the actual size will only ever increase.
	SizeBytes int64 `json:"sizeBytes"`

	// May be updated at will.
	ActivateOnNodes []string `json:"activateOnNodes,omitempty"`
}

type LinearLvStatus struct {
	// Whether the LVM LV has been created.
	Created bool `json:"created"`

	// The current size of the LVM LV.
	SizeBytes int64 `json:"sizeBytes"`

	// The names of the nodes where the LVM LV is active.
	ActiveOnNodes []string `json:"activeOnNodes,omitempty"`

	// The path at which the LVM LV is available on nodes where it is active.
	Path *string `json:"path,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status

type LinearLv struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinearLvSpec   `json:"spec,omitempty"`
	Status LinearLvStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type LinearLvList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinearLv `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinearLv{}, &LinearLvList{})
}
