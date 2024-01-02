// SPDX-License-Identifier: Apache-2.0

package lvm

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

func Command(arg ...string) (string, error) {
	cmd := exec.Command("lvm", arg...)

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DM_DISABLE_UDEV=")

	cmd.Stdin = nil

	output, err := cmd.CombinedOutput()

	switch e := err.(type) {
	case nil:
		log.Printf("LVM command `lvm %s` succeeded:\n%s", strings.Join(arg, " "), output)
	case *exec.ExitError:
		log.Printf("LVM command `lvm %s` failed with status %d:\n%s", strings.Join(arg, " "), e.ExitCode(), output)
	default:
		log.Printf("LVM command `lvm %s` failed to start", strings.Join(arg, " "))
	}

	return string(output), err
}
