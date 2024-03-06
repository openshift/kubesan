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
	name string

	pool *blobPool
}

func NewBlob(blobName string, backingDevicePath string) *Blob {
	return &Blob{
		name: blobName,
		pool: &blobPool{
			name:              blobName,
			backingDevicePath: backingDevicePath,
		},
	}
}

func BlobFromString(s string) (*Blob, error) {
	split := strings.SplitN(s, ":", 3)
	blob := &Blob{
		name: split[0],
		pool: &blobPool{
			name:              split[1],
			backingDevicePath: split[2],
		},
	}
	return blob, nil
}

func (b *Blob) String() string {
	return fmt.Sprintf("%s:%s:%s", b.name, b.pool.name, b.pool.backingDevicePath)
}

func (b *Blob) Name() string {
	return b.name
}

func (b *Blob) lvmThinLvName() string {
	return fmt.Sprintf("%s-thin", b.name)
}

func (b *Blob) lvmThinLvPath() string {
	return fmt.Sprintf("/dev/mapper/%s-%s", config.LvmVgName, strings.ReplaceAll(b.lvmThinLvName(), "-", "--"))
}

func (b *Blob) dmMultipathVolumeName() string {
	return fmt.Sprintf("subprovisioner-%s-dm-multipath", strings.ReplaceAll(b.name, "-", "--"))
}

// The returned path is valid on all nodes in the cluster.
func (b *Blob) dmMultipathVolumePath() string {
	return fmt.Sprintf("/dev/mapper/%s", b.dmMultipathVolumeName())
}

func (bp *blobPool) lvmThinPoolLvName() string {
	return bp.name
}
