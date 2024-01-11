// SPDX-License-Identifier: Apache-2.0

package nbd

import (
	"context"
	"os/exec"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ConnectClient(ctx context.Context, hostname string) (string, error) {
	// TODO: Make idempotent.
	// TODO: Find available device programmatically.

	devicePath := "/dev/nbd15"

	nbdClientCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := nbdClient(nbdClientCtx, hostname, devicePath, "-nonetlink", "-persist")
	if err != nil {
		return "", status.Errorf(
			codes.Internal,
			"failed to connect NBD client device '%s' to hostname '%s': %s: %s",
			devicePath, hostname, err, output,
		)
	}

	return devicePath, nil
}

func DisconnectClient(ctx context.Context, devicePath string) error {
	output, err := nbdClient(context.Background(), "-d", devicePath)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"failed to disconnect NBD client device '%s': %s: %s",
			devicePath, err, output,
		)
	}

	return nil
}

// This returns an error if ctx is canceled, but never attempts to kill the process before it terminates.
func nbdClient(ctx context.Context, arg ...string) (string, error) {
	cmd := exec.Command("nbd-client", arg...)
	cmd.Stdin = nil

	output, err := cmd.CombinedOutput()
	if err := ctx.Err(); err != nil {
		return "", err
	}

	return string(output), err
}
