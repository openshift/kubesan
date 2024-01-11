// SPDX-License-Identifier: Apache-2.0

package volumemanager

import (
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
)

type VolumeManager struct {
	clientset *k8s.Clientset
}

func New(clientset *k8s.Clientset) *VolumeManager {
	return &VolumeManager{clientset: clientset}
}
