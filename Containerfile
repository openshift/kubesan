# SPDX-License-Identifier: Apache-2.0

FROM quay.io/projectquay/golang:1.20 AS builder

WORKDIR /clustered-csi

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN go build -o bin/clustered-csi ./cmd/clustered-csi

FROM quay.io/centos/centos:stream9

RUN dnf install -y lvm2 && dnf clean all

WORKDIR /clustered-csi

COPY --from=builder /clustered-csi/bin/clustered-csi /usr/local/bin/

ENTRYPOINT [ "clustered-csi" ]
