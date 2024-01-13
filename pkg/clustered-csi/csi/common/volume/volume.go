// SPDX-License-Identifier: Apache-2.0

package volume

import (
	"fmt"
	"strings"

	"gitlab.com/clustered-csi/clustered-csi/pkg/clustered-csi/csi/common/config"
	"k8s.io/apimachinery/pkg/types"
)

type Info struct {
	BackingDevicePath string
	PvcUid            types.UID
}

func InfoFromString(s string) *Info {
	split := strings.SplitN(s, ":", 2)
	return &Info{
		BackingDevicePath: split[1],
		PvcUid:            types.UID(split[0]),
	}
}

func (info *Info) ToString() string {
	return fmt.Sprintf("%s:%s", info.PvcUid, info.BackingDevicePath)
}

func (info *Info) ThinLvName() string {
	return fmt.Sprintf("%s-thin", info.PvcUid)
}

func (info *Info) ThinPoolLvName() string {
	return fmt.Sprintf("%s-thin-pool", info.PvcUid)
}

func (info *Info) ThinLvRef() string {
	return fmt.Sprintf("%s/%s", config.VgName, info.ThinLvName())
}

func (info *Info) ThinPoolLvRef() string {
	return fmt.Sprintf("%s/%s", config.VgName, info.ThinPoolLvName())
}
