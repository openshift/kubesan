// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"fmt"
	"strings"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
)

// Some info describing a particular blob.
type Blob struct {
	// The blob's globally-unique name.
	//
	// No two blobs may have the same name.
	Name string

	pool *blobPool
}

type blobPool struct {
	// The name of the Kubernetes PersistentVolume that this blob corresponds to.
	//
	// Every blob is associated to a single PersistentVolume that conceptually "backs" it. Several blobs may
	// correspond to the same PersistentVolume.
	k8sPersistentVolumeName string

	// Path to the shared block device used as storage for this blob.
	backingDevicePath string
}

func NewBlob(blobName string, k8sPersistentVolumeName string, backingDevicePath string) *Blob {
	return &Blob{
		Name: blobName,
		pool: &blobPool{
			k8sPersistentVolumeName: k8sPersistentVolumeName,
			backingDevicePath:       backingDevicePath,
		},
	}
}

func BlobFromString(s string) (*Blob, error) {
	split := strings.SplitN(s, ":", 3)
	blob := &Blob{
		Name: split[0],
		pool: &blobPool{
			k8sPersistentVolumeName: split[1],
			backingDevicePath:       split[2]},
	}
	return blob, nil
}

func (b *Blob) String() string {
	return fmt.Sprintf("%s:%s:%s", b.Name, b.pool.k8sPersistentVolumeName, b.pool.backingDevicePath)
}

func (b *Blob) lvmThinLvName() string {
	return fmt.Sprintf("%s-thin", b.Name)
}

func (b *Blob) lvmThinLvPath() string {
	return fmt.Sprintf("/dev/mapper/%s-%s", config.LvmVgName, strings.ReplaceAll(b.lvmThinLvName(), "-", "--"))
}

func (b *Blob) dmMultipathVolumeName() string {
	return fmt.Sprintf("subprovisioner-%s-dm-multipath", strings.ReplaceAll(b.Name, "-", "--"))
}

// The returned path is valid on all nodes in the cluster.
func (b *Blob) dmMultipathVolumePath() string {
	return fmt.Sprintf("/dev/mapper/%s", b.dmMultipathVolumeName())
}

func (bp *blobPool) lvmThinPoolLvName() string {
	return bp.k8sPersistentVolumeName
}
