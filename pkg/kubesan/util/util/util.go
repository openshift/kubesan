// SPDX-License-Identifier: Apache-2.0

package util

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Hash(strings ...string) string {
	hash := sha1.New()
	for _, s := range strings {
		hash.Write([]byte(s))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// Like os.Symlink, but replaces newname if it is a file, an empty directory, or a symlink (Kubernetes places an empty
// directory at the path where block volumes should be staged/published).
func Symlink(oldname string, newname string) error {
	err := os.Remove(newname)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Symlink(oldname, newname)
}

func RunCommand(command ...string) error {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = nil

	output, err := cmd.CombinedOutput()
	if err != nil {
		return status.Errorf(
			codes.Internal, "command \"%s\" failed: %s: %s",
			strings.Join(command, " "), err, output,
		)
	}

	return nil
}
