// SPDX-License-Identifier: Apache-2.0

package config

import "os"

const (
	Domain  = "kubesan.gitlab.io"
	Version = "v0.4.0"

	K8sNamespace = "kubesan"
)

var (
	Image         = os.Getenv("KUBESAN_IMAGE")
	LocalNodeName = os.Getenv("NODE_NAME")
)
