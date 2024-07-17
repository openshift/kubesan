// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/kubesan/kubesan/pkg/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/csi"
	"k8s.io/client-go/kubernetes/scheme"
)

func badUsage() {
	fmt.Fprintf(os.Stderr, "usage: %s csi-controller-plugin\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "       %s csi-node-plugin\n", os.Args[0])
	os.Exit(2)
}

func init() {
	if err := v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
}

func main() {
	if len(os.Args) != 2 {
		badUsage()
	}

	csiSocketPath := "/run/csi/socket"

	switch os.Args[1] {
	case "csi-controller-plugin":
		err := csi.RunControllerPlugin(csiSocketPath)
		if err != nil {
			log.Fatalln(err)
		}

	case "csi-node-plugin":
		err := csi.RunNodePlugin(csiSocketPath)
		if err != nil {
			log.Fatalln(err)
		}

	default:
		badUsage()
	}
}
