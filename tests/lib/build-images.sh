#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

__build_images() {
    __log_cyan "Building KubeSAN image (localhost/kubesan/kubesan:test)..."
    podman image build -t localhost/kubesan/kubesan:test "${repo_root}"

    __log_cyan "Building test image (localhost/kubesan/test:test)..."
    podman image build -t localhost/kubesan/test:test "${script_dir}/t-data/test-image"
}
export -f __build_images
