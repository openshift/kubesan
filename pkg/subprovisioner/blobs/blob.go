// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Some info describing a particular blob.
type Blob struct {
	// The blob's globally-unique name.
	//
	// No two blobs may have the same name.
	Name string

	// The name of the Kubernetes PersistentVolume that this blob corresponds to.
	//
	// Every blob is associated to a single PersistentVolume that conceptually "backs" it. Several blobs may
	// correspond to the same PersistentVolume.
	K8sPersistentVolumeName string

	// Path to the shared block device used as storage for this blob.
	BackingDevicePath string
}

func BlobFromString(s string) (*Blob, error) {
	split := strings.SplitN(s, ":", 3)
	blob := &Blob{
		Name:                    split[0],
		K8sPersistentVolumeName: split[1],
		BackingDevicePath:       split[2],
	}
	return blob, nil
}

func (b *Blob) String() string {
	return fmt.Sprintf("%s:%s:%s", b.Name, b.K8sPersistentVolumeName, b.BackingDevicePath)
}

func (b *Blob) lvmThinLvName() string {
	return fmt.Sprintf("%s-thin", b.Name)
}

func (b *Blob) lvmThinPoolLvName() string {
	return b.K8sPersistentVolumeName
}

func (bm *BlobManager) atomicUpdateK8sPvForBlob(
	ctx context.Context,
	blob *Blob,
	f func(*corev1.PersistentVolume) error,
) error {
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: blob.K8sPersistentVolumeName}}

	err := k8s.AtomicUpdate(ctx, bm.clientset.CoreV1().RESTClient(), "persistentvolumes", pv, f)
	if err != nil {
		err = status.Errorf(codes.Internal, "failed to update PersistentVolume: %s", err)
	}

	return nil
}
