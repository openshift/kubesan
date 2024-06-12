# SPDX-License-Identifier: Apache-2.0

.PHONY: build
build:
	podman image build --tag subprovisioner:latest .

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: check
check:
	cd tests && ./run.sh all
