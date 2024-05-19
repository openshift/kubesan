// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"gitlab.com/kubesan/kubesan/api/v1alpha1"
)

const (
	Domain  = "kubesan.gitlab.io"
	Version = "v0.4.0"

	Finalizer = Domain + "/finalizer"

	CsiSocketPath = "/run/csi/socket"

	LvmProfileName = "kubesan"
	LvmProfile     = "" +
		"# This file is part of the KubeSAN CSI plugin and may be automatically\n" +
		"# updated. Do not edit!\n" +
		"\n" +
		"activation {\n" +
		"        thin_pool_autoextend_threshold=95\n" +
		"        thin_pool_autoextend_percent=20\n" +
		"}\n"
)

var (
	LocalNodeName = os.Getenv("NODE_NAME")

	Scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))

	utilruntime.Must(v1alpha1.AddToScheme(Scheme))
	//+kubebuilder:scaffold:scheme
}
