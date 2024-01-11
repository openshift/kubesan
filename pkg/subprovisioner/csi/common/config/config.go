// SPDX-License-Identifier: Apache-2.0

package config

const (
	Domain  = "subprovisioner.gitlab.io"
	Version = "0.0.0"

	LvmVgName = "subprovisioner"

	Namespace               = "subprovisioner"
	PvPrimaryAnnotation     = Domain + "/primary"
	PvSecondariesAnnotation = Domain + "/secondaries"
)
