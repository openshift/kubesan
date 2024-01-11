// SPDX-License-Identifier: Apache-2.0

package volume

import (
	"fmt"
	"strings"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/config"
	"k8s.io/apimachinery/pkg/types"
)

type Info struct {
	// Path to the LVM PV corresponding to the shared backing device.
	LvmPvPath string

	// Name of the Kubernetes PersistentVolume for the volume.
	PvName string

	// UID of the Kubernetes PersistentVolumeClaim for the volume.
	PvcUid types.UID
}

func InfoFromString(s string) *Info {
	split := strings.SplitN(s, ":", 3)
	return &Info{
		LvmPvPath: split[2],
		PvName:    split[0],
		PvcUid:    types.UID(split[1]),
	}
}

func (info *Info) ToString() string {
	return fmt.Sprintf("%s:%s:%s", info.PvName, info.PvcUid, info.LvmPvPath)
}

func (info *Info) LvmThinLvName() string {
	return fmt.Sprintf("%s-thin", info.PvcUid)
}

func (info *Info) LvmThinPoolLvName() string {
	return fmt.Sprintf("%s-thin-pool", info.PvcUid)
}

func (info *Info) LvmThinLvRef() string {
	return fmt.Sprintf("%s/%s", config.LvmVgName, info.LvmThinLvName())
}

func (info *Info) LvmThinPoolLvRef() string {
	return fmt.Sprintf("%s/%s", config.LvmVgName, info.LvmThinPoolLvName())
}
