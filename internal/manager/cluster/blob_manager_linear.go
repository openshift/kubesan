// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"gitlab.com/kubesan/kubesan/internal/common/commands"
)

type LinearBlobManager struct {
	vgName string
}

// NewLinearBlobManager returns a BlobManager implemented using LVM's linear
// logical volumes. Linear LVs are fully provisioned and support direct
// ReadWriteMany without NBD when used without LVM's COW snapshots. They are a
// natural fit for use cases that require constant RWX and do not need
// snapshots.
func NewLinearBlobManager(vgName string) BlobManager {
	return &LinearBlobManager{
		vgName: vgName,
	}
}

func (m *LinearBlobManager) CreateBlob(ctx context.Context, name string, sizeBytes int64) error {
	log := log.FromContext(ctx)

	_, err := commands.LvmLvCreateIdempotent(
		"--devicesfile", m.vgName,
		"--activate", "n",
		"--type", "linear",
		"--metadataprofile", "kubesan",
		"--name", name,
		"--size", fmt.Sprintf("%db", sizeBytes),
		m.vgName,
	)
	if err != nil {
		return err
	}

	// TODO recreate if size does not match. This handles the case where a
	// blob was partially created and then reconciled again with a
	// different size. A blob must never be recreated after volume creation
	// has completed since that could lose data!

	// Linear volumes contain the previous contents of the disk, which can
	// be an information leak if multiple users have access to the same
	// Volume Group. Zero the LV to avoid security issues.
	LvmLvTagZeroed := "kubesan.gitlab.io/zeroed=true"
	hasTag, err := commands.LvmLvHasTag(m.vgName, name, LvmLvTagZeroed)
	if err != nil {
		return err
	}
	if !hasTag {
		err = commands.WithLvmLvActivated(m.vgName, name, func() error {
			path := fmt.Sprintf("/dev/%s/%s", m.vgName, name)
			log.Info("CreateBlob zeroing LV", "path", path)
			_, err := commands.RunOnHost("blkdiscard", "--zeroout", path)
			return err
		})
		if err != nil {
			return err
		}

		err = commands.LvmLvAddTag(m.vgName, name, LvmLvTagZeroed)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *LinearBlobManager) RemoveBlob(ctx context.Context, name string) error {
	_, err := commands.LvmLvRemoveIdempotent(
		"--devicesfile", m.vgName,
		fmt.Sprintf("%s/%s", m.vgName, name),
	)
	return err
}
