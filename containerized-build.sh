# SPDX-License-Identifier: Apache-2.0

set -e -x

#Build kubesan image
podman build -t kubesan:latest .
