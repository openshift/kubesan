# SPDX-License-Identifier: Apache-2.0

.PHONY: build
build:
	podman image build -t quay.io/clustered-csi/clustered-csi:0.0.0 .

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...
