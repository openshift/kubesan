// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type SnapshotSpec struct {
	// Should be set from creation and never updated.
	SourceVolume string `json:"sourceVolume"`
}

type SnapshotStatus struct {
	// Whether the snapshot has been created.
	Created bool `json:"created"`

	// The time at which the snapshot was created.
	CreationTime *metav1.Time `json:"creationTime"`

	// The size of the snapshot.
	SizeBytes *int64 `json:"sizeBytes"`

	// The file system type of the snapshot. `nil` if a snapshot of a block volume.
	FsType *string `json:"fsType,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status

type Snapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotSpec   `json:"spec,omitempty"`
	Status SnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Snapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Snapshot{}, &SnapshotList{})
}
