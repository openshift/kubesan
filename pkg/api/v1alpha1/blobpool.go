// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&BlobPool{}, &BlobPoolList{})
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster

type BlobPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BlobPoolSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// BlobPoolList contains a list of BlobPool
type BlobPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlobPool `json:"items"`
}

type BlobPoolSpec struct {
	// +optional
	Blobs []string `json:"blobs,omitempty"`

	// +optional
	// The node on which the LVM thin *pool* LV backing this blob pool is active, if any.
	ActiveOnNode *string `json:"activeOnNode,omitempty"`

	// +optional
	Holders []BlobPoolHolder `json:"holders,omitempty"`
}

type BlobPoolHolder struct {
	Blob   string `json:"blob"`
	Node   string `json:"node"`
	Cookie string `json:"cookie"`
}

func (spec *BlobPoolSpec) BlobsWithHolders() []string {
	var blobs []string
	for _, holder := range spec.Holders {
		blobs = slices.AppendUnique(blobs, holder.Blob)
	}
	return blobs
}

func (spec *BlobPoolSpec) NodesWithHoldersForBlob(blob string) []string {
	var nodes []string
	for _, holder := range spec.Holders {
		if holder.Blob == blob {
			nodes = slices.AppendUnique(nodes, holder.Node)
		}
	}
	return nodes
}

func (spec *BlobPoolSpec) HasHolderForBlob(blob string) bool {
	return slices.Any(spec.Holders, func(h BlobPoolHolder) bool { return h.Blob == blob })
}

func (spec *BlobPoolSpec) HasHolderOnNode(node string) bool {
	return slices.Any(spec.Holders, func(h BlobPoolHolder) bool { return h.Node == node })
}

func (spec *BlobPoolSpec) HasHolderForBlobOnNode(blob string, node string) bool {
	return slices.Any(spec.Holders, func(h BlobPoolHolder) bool { return h.Blob == blob && h.Node == node })
}

func (spec *BlobPoolSpec) HasHolder(blob string, node string, cookie string) bool {
	holder := BlobPoolHolder{Blob: blob, Node: node, Cookie: cookie}
	return slices.Contains(spec.Holders, holder)
}

func (spec *BlobPoolSpec) AddHolder(blob string, node string, cookie string) {
	holder := BlobPoolHolder{Blob: blob, Node: node, Cookie: cookie}
	spec.Holders = slices.AppendUnique(spec.Holders, holder)
}

func (spec *BlobPoolSpec) RemoveHolder(blob string, node string, cookie string) {
	holder := BlobPoolHolder{Blob: blob, Node: node, Cookie: cookie}
	spec.Holders = slices.Remove(spec.Holders, holder)
}
