// SPDX-License-Identifier: Apache-2.0

package lvm

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
)

// This returns an error if ctx is canceled, but never attempts to kill the process before it terminates.
func Command(ctx context.Context, command string, arg ...string) (string, error) {
	fullArgs := append([]string{command}, arg...)
	cmd := exec.Command("lvm", fullArgs...)

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

	if err := ctx.Err(); err != nil {
		return "", err
	}

	return string(output), err
}

// Ignores lvcreate errors due to the LV already existing.
func IdempotentLvCreate(ctx context.Context, arg ...string) (string, error) {
	output, err := Command(ctx, "lvcreate", arg...)

	if err != nil && strings.Contains(strings.ToLower(output), "already exists in volume group") {
		err = nil
	}

	return output, nil
}

// Ignores lvremove errors due to the LV not existing.
func IdempotentLvRemove(ctx context.Context, arg ...string) (string, error) {
	output, err := Command(ctx, "lvremove", arg...)

	if err != nil && strings.Contains(strings.ToLower(output), "failed to find logical volume") {
		err = nil
	}

	return output, nil
}
