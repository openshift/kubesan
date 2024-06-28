// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Some info describing a particular blob.
type Blob struct {
	// The blob's globally-unique name.
	//
	// No two blobs may have the same name.
	name string

	pool *blobPool
}

func NewBlob(blobName string, backingVolumeGroup string) *Blob {
	return &Blob{
		name: blobName,
		pool: &blobPool{
			name:               blobName,
			backingVolumeGroup: backingVolumeGroup,
		},
	}
}

func BlobFromString(s string) (*Blob, error) {
	split := strings.Split(s, ":")
	if len(split) != 3 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid blob name")
	}
	blob := &Blob{
		name: split[0],
		pool: &blobPool{
			name:               split[1],
			backingVolumeGroup: split[2],
		},
	}
	return blob, nil
}

func (b *Blob) String() string {
	return fmt.Sprintf("%s:%s:%s", b.name, b.pool.name, b.pool.backingVolumeGroup)
}

func (b *Blob) Name() string {
	return b.name
}

func (b *Blob) lvmThinLvName() string {
	return fmt.Sprintf("%s-thin", b.name)
}

func (b *Blob) lvmThinLvPath() string {
	return fmt.Sprintf("/dev/%s/%s", b.pool.backingVolumeGroup, b.lvmThinLvName())
}

func (b *Blob) dmMultipathVolumeName() string {
	return fmt.Sprintf("%s-%s-dm-multipath", strings.ReplaceAll(b.pool.backingVolumeGroup, "-", "--"), strings.ReplaceAll(b.name, "-", "--"))
}

// The returned path is valid on all nodes in the cluster.
func (b *Blob) dmMultipathVolumePath() string {
	return fmt.Sprintf("/dev/mapper/%s", b.dmMultipathVolumeName())
}

func (bp *blobPool) lvmThinPoolLvName() string {
	return bp.name
}
