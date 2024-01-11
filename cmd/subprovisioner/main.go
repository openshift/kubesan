// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi"
)

func badUsage() {
	fmt.Fprintf(os.Stderr, "usage: %s csi-controller-plugin\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "       %s csi-node-plugin\n", os.Args[0])
	os.Exit(2)
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
