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
	return err
}

func (m *FatBlobManager) RemoveBlob(name string) error {
	_, err := commands.LvmLvRemoveIdempotent(
		"--devicesfile", m.vgName,
		fmt.Sprintf("%s/%s", m.vgName, name),
	)
	return err
}
