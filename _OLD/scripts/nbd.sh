#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o pipefail -o nounset -o xtrace

command=$1

case "${command}" in
    server)
        device_to_export=$2

        while [[ ! -b "$device_to_export" ]]; do
            sleep 1
        done

        exec qemu-nbd --cache=none --format=raw --persistent --shared=0 "$device_to_export"
        ;;
esac
