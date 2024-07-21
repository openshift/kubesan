// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"io"
	"os"
)

const (
	Domain  = "kubesan.gitlab.io"
	Version = "v0.4.0"
)

var ErrCannotDetermineNamespace = errors.New("could not determine namespace from service account or K8S_NAMESPACE env var")

var (
	Image         = os.Getenv("KUBESAN_IMAGE")
	LocalNodeName = os.Getenv("NODE_NAME")
	K8sNamespace  string
)

func init() {
	K8sNamespace = getK8sNamespace()
}

func getK8sNamespace() string {
	namespace, present := os.LookupEnv("K8S_NAMESPACE")
	if present {
		return namespace
	}

	fi, err := os.Open("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		panic(errors.Join(ErrCannotDetermineNamespace, err))
	}
	defer func(fi *os.File) {
		if err := fi.Close(); err != nil {
			panic(errors.Join(ErrCannotDetermineNamespace, err))
		}
	}(fi)

	data, err := io.ReadAll(fi)
	if err != nil {
		panic(errors.Join(ErrCannotDetermineNamespace, err))
	}

	return string(data)
}
