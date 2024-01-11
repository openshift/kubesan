// SPDX-License-Identifier: Apache-2.0

package lvm

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/volume"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// This returns an error if ctx is canceled, but never attempts to kill the process before it terminates.
func Command(ctx context.Context, command string, arg ...string) (string, error) {
	fullArgs := append([]string{command}, arg...)
	cmd := exec.Command("lvm", fullArgs...)

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DM_DISABLE_UDEV=")

	cmd.Stdin = nil

	output, err := cmd.CombinedOutput()

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

func StartVgLockspace(ctx context.Context, lvmPvPath string) error {
	args := []string{"--devices", lvmPvPath, "--lock-start", config.LvmVgName}

	_, err := Command(ctx, "vgchange", args...)
	if err != nil {
		// oftentimes trying again works (TODO: figure out why)
		output, err := Command(ctx, "vgchange", args...)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to start vg lockspace: %s: %s", err, output)
		}
	}

	return nil
}

func GetLvPath(ctx context.Context, info *volume.Info) (string, error) {
	output, err := Command(
		ctx,
		"lvs",
		"--devices", info.LvmPvPath,
		"--options", "lv_path",
		"--noheadings",
		info.LvmThinLvRef(),
	)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to get path to LVM LV: %s: %s", err, output)
	}

	return strings.TrimSpace(output), nil
}
