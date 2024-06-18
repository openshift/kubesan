// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"

	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/config"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/slices"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

type blobPool struct {
	// Matches the name of the pool's Kubernetes BlobPool CRD object.
	name string

	// Name of the shared VG used as storage for this blob.
	backingVolumeGroup string
}

type blobPoolCrd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec blobPoolCrdSpec `json:"spec"`
}

type blobPoolCrdSpec struct {
	Blobs []string `json:"blobs"`

	// The node on which the LVM thin *pool* LV backing this blob pool is active, if any.
	ActiveOnNode *string          `json:"activeOnNode"`
	Holders      []blobPoolHolder `json:"holders"`
}

type blobPoolHolder struct {
	Blob   string `json:"blob"`
	Node   string `json:"node"`
	Cookie string `json:"cookie"`
}

func (in *blobPoolCrd) DeepCopyObject() runtime.Object {
	out := blobPoolCrd{}

	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta

	out.Spec = blobPoolCrdSpec{}
	out.Spec.Blobs = append([]string{}, in.Spec.Blobs...)
	if in.Spec.ActiveOnNode != nil {
		copy := *in.Spec.ActiveOnNode
		out.Spec.ActiveOnNode = &copy
	}
	out.Spec.Holders = append([]blobPoolHolder{}, in.Spec.Holders...)

	return &out
}

func (spec *blobPoolCrdSpec) blobsWithHolders() []string {
	var blobs []string
	for _, holder := range spec.Holders {
		blobs = slices.AppendUnique(blobs, holder.Blob)
	}
	return blobs
}

func (spec *blobPoolCrdSpec) nodesWithHoldersForBlob(blob string) []string {
	var nodes []string
	for _, holder := range spec.Holders {
		if holder.Blob == blob {
			nodes = slices.AppendUnique(nodes, holder.Node)
		}
	}
	return nodes
}

func (spec *blobPoolCrdSpec) hasHolderForBlob(blob string) bool {
	return slices.Any(spec.Holders, func(h blobPoolHolder) bool { return h.Blob == blob })
}

func (spec *blobPoolCrdSpec) hasHolderOnNode(node string) bool {
	return slices.Any(spec.Holders, func(h blobPoolHolder) bool { return h.Node == node })
}

func (spec *blobPoolCrdSpec) hasHolderForBlobOnNode(blob string, node string) bool {
	return slices.Any(spec.Holders, func(h blobPoolHolder) bool { return h.Blob == blob && h.Node == node })
}

func (spec *blobPoolCrdSpec) hasHolder(blob string, node string, cookie string) bool {
	holder := blobPoolHolder{Blob: blob, Node: node, Cookie: cookie}
	return slices.Contains(spec.Holders, holder)
}

func (spec *blobPoolCrdSpec) addHolder(blob string, node string, cookie string) {
	holder := blobPoolHolder{Blob: blob, Node: node, Cookie: cookie}
	spec.Holders = slices.AppendUnique(spec.Holders, holder)
}

func (spec *blobPoolCrdSpec) removeHolder(blob string, node string, cookie string) {
	holder := blobPoolHolder{Blob: blob, Node: node, Cookie: cookie}
	spec.Holders = slices.Remove(spec.Holders, holder)
}

func (bm *BlobManager) createBlobPoolCrd(ctx context.Context, poolName string, poolSpec *blobPoolCrdSpec) error {
	crd := blobPoolCrd{
		ObjectMeta: metav1.ObjectMeta{Name: poolName},
		Spec:       *poolSpec,
	}
	if crd.Spec.Blobs == nil {
		crd.Spec.Blobs = []string{}
	}

	if crd.Spec.Holders == nil {
		crd.Spec.Holders = []blobPoolHolder{}
	}

	req := bm.crdRest.Post().Resource("blobpools").Body(&crd).
		VersionedParams(&metav1.CreateOptions{}, scheme.ParameterCodec)

	err := req.Do(ctx).Error()
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (bm *BlobManager) getBlobPoolCrd(ctx context.Context, poolName string) (*blobPoolCrdSpec, error) {
	req := bm.crdRest.Get().Resource("blobpools").Name(poolName).
		VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec)

	crd := blobPoolCrd{}
	err := req.Do(ctx).Into(&crd)
	if err != nil {
		return nil, err
	}

	return &crd.Spec, nil
}

func (bm *BlobManager) deleteBlobPoolCrd(ctx context.Context, poolName string) error {
	req := bm.crdRest.Delete().Resource("blobpools").Name(poolName).
		VersionedParams(&metav1.DeleteOptions{}, scheme.ParameterCodec)

	err := req.Do(ctx).Error()
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}

func init() {
	builder := runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		gv := schema.GroupVersion{Group: config.Domain, Version: "v1alpha1"}
		scheme.AddKnownTypeWithName(gv.WithKind("BlobPool"), &blobPoolCrd{})
		metav1.AddToGroupVersion(scheme, gv)
		return nil
	})

	builder.AddToScheme(scheme.Scheme)
}
