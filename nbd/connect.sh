#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -ex

device_symlink=$1
server_host=$2

# This relies on nbd-client with netlink support to pick the next available
# nbd block device, which it outputs in a line "Connected /dev/nbdX"
details=$(nbd-client "$server_host" -persist)
nbd_device_path=$(printf '%s\n' "$details" | sed -n 's/^Connected //p')
ln -fs "$nbd_device_path" "$device_symlink"
