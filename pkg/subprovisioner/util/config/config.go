// SPDX-License-Identifier: Apache-2.0

package config

import "os"

const (
	Domain  = "subprovisioner.gitlab.io"
	Version = "0.0.1"

	K8sNamespace = "subprovisioner"
	LvmVgName    = "subprovisioner"
)

var (
	Image         = os.Getenv("SUBPROVISIONER_IMAGE")
	LocalNodeName = os.Getenv("NODE_NAME")
)
