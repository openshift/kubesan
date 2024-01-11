// SPDX-License-Identifier: Apache-2.0

package volumemanager

import (
	"fmt"
	"strings"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"k8s.io/apimachinery/pkg/types"
)

// Some info describing a particular volume.
type Volume struct {
	// The UID of the Kubernetes PersistentVolumeClaim object for the volume.
	//
	// This uniquely identifies the volume.
	K8sPvcUid types.UID

	// The name of the Kubernetes PersistentVolume object for the volume.
	//
	// Leave empty if the PersistentVolume for the volume does not yet exist.
	K8sPvName string

	// Path to the shared block device used as storage for this volume.
	BackingDevicePath string
}

func VolumeFromString(s string) (*Volume, error) {
	split := strings.SplitN(s, ":", 3)
	vol := &Volume{
		K8sPvcUid:         types.UID(split[0]),
		K8sPvName:         split[1],
		BackingDevicePath: split[2],
	}
	return vol, nil
}

func (v *Volume) ToString() string {
	return fmt.Sprintf("%s:%s:%s", v.K8sPvcUid, v.K8sPvName, v.BackingDevicePath)
}

func (v *Volume) lvmThinLvName() string {
	return fmt.Sprintf("%s-thin", v.K8sPvcUid)
}

func (v *Volume) lvmThinPoolLvName() string {
	return fmt.Sprintf("%s-thin-pool", v.K8sPvcUid)
}

func (v *Volume) lvmThinLvRef() string {
	return fmt.Sprintf("%s/%s", config.LvmVgName, v.lvmThinLvName())
}

func (v *Volume) lvmThinPoolLvRef() string {
	return fmt.Sprintf("%s/%s", config.LvmVgName, v.lvmThinPoolLvName())
}
