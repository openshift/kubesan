# SPDX-License-Identifier: Apache-2.0

FROM quay.io/projectquay/golang:1.20 AS builder

WORKDIR /subprovisioner

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN go build -o bin/subprovisioner ./cmd/subprovisioner

FROM quay.io/centos/centos:stream9

RUN dnf install -y lvm2 && dnf clean all

WORKDIR /subprovisioner

COPY --from=builder /subprovisioner/bin/subprovisioner /usr/local/bin/

ENTRYPOINT [ "subprovisioner" ]
