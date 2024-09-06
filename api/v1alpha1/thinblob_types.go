// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make generate" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ThinBlobSpec struct {
	VgName        string           `json:"vgName"`
	Contents      ThinBlobContents `json:"contents"`
	SizeBytes     int64            `json:"sizeBytes"`
	ReadOnly      bool             `json:"readOnly"`
	AttachToNodes []string         `json:"attachToNodes,omitempty"`
}

type ThinBlobContents struct {
	Empty      *ThinBlobContentsEmpty      `json:"empty,omitempty"`
	Filesystem *ThinBlobContentsFilesystem `json:"filesystem,omitempty"`
	Copy       *ThinBlobContentsCopy       `json:"copy,omitempty"`
}

type ThinBlobContentsEmpty struct {
}

type ThinBlobContentsFilesystem struct {
	FsType string `json:"fsType"`
}

type ThinBlobContentsCopy struct {
	Blob string `json:"blob"`
}

type ThinBlobStatus struct {
	State              ThinBlobState        `json:"state"`
	ThinPoolLvName     *string              `json:"thinPoolLvName"`
	SizeBytes          int64                `json:"sizeBytes"`
	FsType             *string              `json:"fsType,omitempty"`
	DataCopyInProgress bool                 `json:"dataCopyInProgress,omitempty"`
	Attachments        []ThinBlobAttachment `json:"attachments,omitempty"`
}

type ThinBlobState struct {
	Creating  *ThinBlobStateCreating  `json:"creating,omitempty"`
	Ready     *ThinBlobStateReady     `json:"created,omitempty"`
	Expanding *ThinBlobStateExpanding `json:"expanding,omitempty"`
	Migrating *ThinBlobStateMigrating `json:"migrating,omitempty"`
}

type ThinBlobStateCreating struct {
}

type ThinBlobStateReady struct {
}

type ThinBlobStateExpanding struct {
}

type ThinBlobStateMigrating struct {
	NewLocalNode string `json:"newLocalNode"`
}

type ThinBlobAttachment struct {
	Node  string                  `json:"node"`
	State ThinBlobAttachmentState `json:"state"`
}

type ThinBlobAttachmentState struct {
	Attaching      *ThinBlobAttachmentStateAttaching      `json:"attaching,omitempty"`
	AttachedLocal  *ThinBlobAttachmentStateAttachedLocal  `json:"attachedLocal,omitempty"`
	AttachedRemote *ThinBlobAttachmentStateAttachedRemote `json:"attachedRemote,omitempty"`
	Detaching      *ThinBlobAttachmentStateDetaching      `json:"detaching,omitempty"`
	Suspending     *ThinBlobAttachmentStateSuspending     `json:"suspending,omitempty"`
	Suspended      *ThinBlobAttachmentStateSuspended      `json:"suspended,omitempty"`
	Resuming       *ThinBlobAttachmentStateResuming       `json:"resuming,omitempty"`
}

type ThinBlobAttachmentStateAttaching struct {
}

type ThinBlobAttachmentStateAttachedLocal struct {
}

type ThinBlobAttachmentStateAttachedRemote struct {
}

type ThinBlobAttachmentStateDetaching struct {
}

type ThinBlobAttachmentStateSuspending struct {
}

type ThinBlobAttachmentStateSuspended struct {
}

type ThinBlobAttachmentStateResuming struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status

type ThinBlob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThinBlobSpec   `json:"spec,omitempty"`
	Status ThinBlobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ThinBlobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThinBlob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ThinBlob{}, &ThinBlobList{})
}
