// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/clustered-csi/clustered-csi/pkg/clustered-csi/csi"
)

func badUsage() {
	fmt.Fprintf(os.Stderr, "usage: %s csi-controller-plugin <image>\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "       %s csi-node-plugin <node_name> <image>\n", os.Args[0])
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		badUsage()
	}

	csiSocketPath := "/run/csi/socket"

	switch os.Args[1] {
	case "csi-controller-plugin":
		if len(os.Args) != 3 {
			badUsage()
		}

		err := csi.RunControllerPlugin(csiSocketPath, os.Args[2])
		if err != nil {
			log.Fatalln(err)
		}

	case "csi-node-plugin":
		if len(os.Args) != 4 {
			badUsage()
		}

		err := csi.RunNodePlugin(csiSocketPath, os.Args[2], os.Args[3])
		if err != nil {
			log.Fatalln(err)
		}

	default:
		badUsage()
	}
}
