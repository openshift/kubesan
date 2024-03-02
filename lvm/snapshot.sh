#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -ex

backing_device_path=$1
lvm_source_lv_name=$2
lvm_target_lv_name=$3

export DM_DISABLE_UDEV=

__lockstart() {
    lvm vgchange \
        --devices "$backing_device_path" \
        --lock-start \
        subprovisioner
}

# oftentimes trying a second time works (TODO: figure out why)
__lockstart || __lockstart

output=$(
    lvm lvcreate \
        --devices "$backing_device_path" \
        --name "$lvm_target_lv_name" \
        --snapshot \
        --setactivationskip n \
        "subprovisioner/$lvm_source_lv_name" \
        2>&1 \
        | tee /dev/stderr
    ) || grep -i "already exists in volume group" <<< "$output"

lvm lvchange \
    --devices "$backing_device_path" \
    --activate n \
    "subprovisioner/$lvm_target_lv_name"
