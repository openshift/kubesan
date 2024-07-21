# SPDX-License-Identifier: Apache-2.0

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
CONTROLLER_TOOLS_VERSION := 0.15.0
IMG ?= kubesan:latest

.PHONY: build
build:
	podman image build --tag $(IMG) .

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: check
check:
	cd tests && ./run.sh all

generate: controller-gen
	$(CONTROLLER_GEN) crd object paths="./..."
	rm deploy/kubernetes/01-crd.yaml
	cat $(PROJECT_DIR)/config/crd/kubesan.gitlab.io_blobpools.yaml >> deploy/kubernetes/01-crd.yaml
	rm -r $(PROJECT_DIR)/config/crd


CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION))

# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || echo "Downloading $(2)"; GOBIN=$(PROJECT_DIR)/bin go install -mod=readonly $(2)
endef
