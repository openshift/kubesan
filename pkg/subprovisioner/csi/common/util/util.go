// SPDX-License-Identifier: Apache-2.0

package util

import (
	"errors"
	"os"

	"k8s.io/client-go/kubernetes"
)

type Clientset struct {
	*kubernetes.Clientset
}

// Like os.Symlink, but replaces newname if it is a file or an empty directory (Kubernetes may place a directory at the
// path where block volumes should be staged/published).
func Symlink(oldname, newname string) error {
	err := os.Remove(newname)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Symlink(oldname, newname)
}
