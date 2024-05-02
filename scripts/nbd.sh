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

    client-connect)
        device_symlink=$2
        server_host=$3

        # This relies on nbd-client with netlink support to pick the next available
        # nbd block device, which it outputs in a line "Connected /dev/nbdX"
        nbd_device_path=$(
            nbd-client "$server_host" --persist --connections 8 |
                sed -n 's/^Connected //p'
            )

        ln -fs "$nbd_device_path" "$device_symlink"
        ;;

    client-disconnect)
        device_symlink=$2

        exec nbd-client -d "$( readlink -e "$device_symlink" )"
        ;;
esac
