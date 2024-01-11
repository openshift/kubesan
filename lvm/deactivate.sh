#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -ex

backing_device_path=$1
lvm_thin_lv_ref=$2
lvm_thin_pool_lv_ref=$3

export DM_DISABLE_UDEV=

lvm lvchange \
    --devices "$backing_device_path" \
    --activate n \
    "$lvm_thin_lv_ref"

lvm lvchange \
    --devices "$backing_device_path" \
    --activate n \
    "$lvm_thin_pool_lv_ref"
