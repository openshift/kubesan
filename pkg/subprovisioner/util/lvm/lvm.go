// SPDX-License-Identifier: Apache-2.0

package lvm

import (
	"context"
	"os"
	"os/exec"
	"strings"

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

	stdout, err := cmd.Output()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			return "", status.Errorf(
				codes.Internal, "command \"lvm %s\" failed: %s: %s",
				strings.Join(fullArgs, " "), err, exiterr.Stderr,
			)
		} else {
			return "", status.Errorf(
				codes.Internal, "command \"lvm %s\" failed: %s",
				strings.Join(fullArgs, " "), err,
			)
		}

	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	return string(stdout), err
}
