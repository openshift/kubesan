// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"fmt"

	"gitlab.com/kubesan/kubesan/internal/common/commands"
)

type FatBlobManager struct {
	vgName string
}

// NewFatBlobManager returns a BlobManager implemented using LVM's linear
// logical volumes. Linear LVs are fully provisioned (hence the name "fat") and
// support direct ReadWriteMany without NBD when used without LVM's COW
// snapshots. They are a natural fit for use cases that require constant RWX
// and do not need snapshots.
func NewFatBlobManager(vgName string) BlobManager {
	return &FatBlobManager{
		vgName: vgName,
	}
}

func (m *FatBlobManager) CreateBlob(name string, sizeBytes int64) error {
	_, err := commands.LvmLvCreateIdempotent(
		"--devicesfile", m.vgName,
		"--activate", "n",
		"--type", "linear",
		"--metadataprofile", "kubesan",
		"--name", name,
		"--size", fmt.Sprintf("%db", sizeBytes),
		m.vgName,
	)

	// TODO recreate if size does not match. This handles the case where a
	// blob was partially created and then reconciled again with a
	// different size. A blob must never be recreated after volume creation
	// has completed since that could lose data!

	// TODO zero the LV (linear LVs are not automatically zero-initialized)
	// https://gitlab.com/kubesan/kubesan/-/issues/63
	return err
}

func (m *FatBlobManager) RemoveBlob(name string) error {
	_, err := commands.LvmLvRemoveIdempotent(
		"--devicesfile", m.vgName,
		fmt.Sprintf("%s/%s", m.vgName, name),
	)
	return err
}
