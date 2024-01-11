#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -ex

# TODO: We currently base the lvmlockd host_id on the last 10 bits of the host's
# IP. This may not always lead to unique IDs and only works with up to 1024
# nodes. Find a better approach.

host_ip=$1
IFS=. read -r _ _ c d <<< "$host_ip"
lvm_host_id=$(( ((c * 256 + d) & 0x3ff) + 1 ))

exec lvmlockd \
    --daemon-debug \
    --gl-type sanlock \
    --host-id "$lvm_host_id"
