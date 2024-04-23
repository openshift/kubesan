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
	fullArgs := append([]string{"--target", "1", "--all", "lvm", command}, arg...)
	cmd := exec.Command("nsenter", fullArgs...)

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

func WriteProfile(name string, contents string) error {
	// This should never happen but be extra careful since the name is used
	// to build a path outside the container's mount namespace and
	// container escapes must be prevented.
	if strings.ContainsAny(name, "/") {
		return status.Errorf(codes.Internal, "lvm profile name \"%s\" must not contain a '/' character", name)
	}
	if name == ".." {
		return status.Errorf(codes.Internal, "lvm profile name \"%s\" must not be \"..\"", name)
	}

	// This process runs in the node's PID namespace, so the node's root
	// directory is accessible through the init process.
	dir := "/proc/1/root/etc/lvm/profile"
	path := dir + "/" + name + ".profile"

	f, err := os.CreateTemp(dir, "subprovisioner-*")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create temporary file for lvm profile \"%s\": %s", name, err)
	}
	fClosed := false
	fRenamed := false
	defer func() {
		if !fClosed {
			f.Close()
		}
		if !fRenamed {
			os.Remove(f.Name())
		}
	}()

	if err := f.Chmod(0644); err != nil {
		return status.Errorf(codes.Internal, "failed to chmod lvm profile \"%s\": %s", name, err)
	}
	if _, err := f.WriteString(contents); err != nil {
		return status.Errorf(codes.Internal, "failed to write lvm profile \"%s\": %s", name, err)
	}
	if err := f.Close(); err != nil {
		return status.Errorf(codes.Internal, "failed to close lvm profile \"%s\": %s", name, err)
	}
	fClosed = true
	if err := os.Rename(f.Name(), path); err != nil {
		return status.Errorf(codes.Internal, "failed to rename lvm profile \"%s\": %s", name, err)
	}
	fRenamed = true
	return nil
}
