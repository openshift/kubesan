#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -ex

backing_device_path=$1
lvm_thin_pool_lv_ref=$2
lvm_thin_lv_ref=$3

export DM_DISABLE_UDEV=

__lockstart() {
    lvm vgchange \
        --devices "$backing_device_path" \
        --lock-start \
        subprovisioner
}

# oftentimes trying a second time works (TODO: figure out why)
__lockstart || __lockstart

lvm lvchange \
    --devices "$backing_device_path" \
    --activate ey \
    "$lvm_thin_pool_lv_ref"

lvm lvchange \
    --devices "$backing_device_path" \
    --activate ey \
    "$lvm_thin_lv_ref"
