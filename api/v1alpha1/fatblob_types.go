// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type FatBlobSpec struct {
	// Should be set from creation and never updated.
	VgName string `json:"vgName"`

	// Should be set from creation and never updated.
	ReadOnly bool `json:"readOnly"`

	// Must be positive and a multiple of 512. May be updated at will, but the actual size will only ever increase.
	SizeBytes int64 `json:"sizeBytes"`

	// May be updated at will.
	AttachToNodes []string `json:"attachToNodes,omitempty"`
}

type FatBlobStatus struct {
	// Whether the blob has been created.
	Created bool `json:"created"`

	// The current size of the blob.
	SizeBytes int64 `json:"sizeBytes"`

	// The nodes to which the blob is attached.
	AttachedToNodes []string `json:"attachedToNodes,omitempty"`

	// The path at which the blob is available on nodes to which it is attached.
	Path *string `json:"path,omitempty"`
}

func (f *FatBlobStatus) GetPath() string {
	if f.Path == nil {
		return ""
	} else {
		return *f.Path
	}
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status

type FatBlob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FatBlobSpec   `json:"spec,omitempty"`
	Status FatBlobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type FatBlobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FatBlob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FatBlob{}, &FatBlobList{})
}
