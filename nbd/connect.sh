#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -ex

device_symlink=$1
server_host=$2

nbd_device_path=/dev/nbd15  # TODO: actually find available device

ln -fs "$nbd_device_path" "$device_symlink"

exec nbd-client "$server_host" "$nbd_device_path" -persist
