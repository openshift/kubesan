// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kubesan/kubesan/internal/csi"
	"github.com/kubesan/kubesan/internal/manager"
)

func badUsage() {
	fmt.Fprintf(os.Stderr, "usage: kubesan csi-controller-plugin\n")
	fmt.Fprintf(os.Stderr, "       kubesan csi-node-plugin\n")
	fmt.Fprintf(os.Stderr, "       kubesan cluster-controller-manager\n")
	fmt.Fprintf(os.Stderr, "       kubesan node-controller-manager\n")
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		badUsage()
	}

	var err error

	switch os.Args[1] {
	case "csi-controller-plugin":
		err = csi.RunControllerPlugin()

	case "csi-node-plugin":
		err = csi.RunNodePlugin()

	case "cluster-controller-manager":
		err = manager.RunClusterControllers()

	case "node-controller-manager":
		err = manager.RunNodeControllers()

	default:
		badUsage()
	}

	if err != nil {
		log.Fatalln(err)
	}
}
