// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"errors"
	"os"
)

// Like os.Symlink, but replaces newname if it is a file, an empty directory, or a symlink (Kubernetes places an empty
// directory at the path where block volumes should be staged/published).
func Symlink(oldname string, newname string) error {
	err := os.Remove(newname)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Symlink(oldname, newname)
}
