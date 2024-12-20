ARG TARGETOS
ARG TARGETARCH
ARG TARGETPLATFORM
FROM golang:1.22 AS builder

WORKDIR /kubesan

# Verfiy the vendoring is clean
COPY go.mod go.sum ./
COPY vendor vendor/
RUN go mod verify

# Copy the source
COPY api/ api/
COPY cmd/ cmd/
COPY deploy/ deploy/
COPY hack/ hack/
COPY internal/ internal/

# We set GOOS and GOARCH so go cross-compiles to the correct os and arch
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -mod=vendor --ldflags "-s -w" -a -o bin/kubesan cmd/main.go

# CentOS Stream 9 doesn't provide package nbd
# FROM quay.io/centos/centos:stream9
# We use --platform=$TARGETPLATFORM to pull the correct arch for
# the base image. This is needed for multi-arch builds
FROM --platform=$TARGETPLATFORM fedora:latest

# util-linux-core, e2fsprogs, and xfsprogs are for Filesystem volume support where
# blkid(8) and mkfs are required by k8s.io/mount-utils.
RUN dnf update -y && dnf install --nodocs --noplugins -qy nbd qemu-img util-linux-core e2fsprogs xfsprogs && dnf clean all

WORKDIR /kubesan

COPY --from=builder /kubesan/bin/kubesan bin/

ENTRYPOINT [ "/kubesan/bin/kubesan" ]
