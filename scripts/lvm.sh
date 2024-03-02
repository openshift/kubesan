#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o pipefail -o nounset -o xtrace

command=$1
backing_device_path=$2
lvm_thin_pool_lv_name=$3
lvm_thin_lv_name=$4

export DM_DISABLE_UDEV=

# Usage: __run_ignoring_error <error_regex> <cmd...>
__run_ignoring_error() {
    local output
    output=$( "${@:2}" 2>&1 | tee /dev/stderr ) || grep -iE "$1" <<< "$output"
}

__lockstart() {
    lvm vgchange \
        --devices "$backing_device_path" \
        --lock-start \
        subprovisioner
}

# ensure LVM VG lockspace is started (oftentimes it first fails but works on the
# second try; TODO: figure out why)
__lockstart || __lockstart

case "${command}" in
    create)
        size_bytes=$5

        # create LVM thin pool LV

        __run_ignoring_error "already exists in volume group" \
            lvm lvcreate \
            --devices "$backing_device_path" \
            --activate n \
            --type thin-pool \
            --name "$lvm_thin_pool_lv_name" \
            --size "$(( size_bytes * 2 ))b" \
            subprovisioner

	    # create LVM thin LV

        __run_ignoring_error "already exists in volume group" \
            lvm lvcreate \
            --devices "$backing_device_path" \
            --type "thin" \
            --name "$lvm_thin_lv_name" \
            --thinpool "$lvm_thin_pool_lv_name" \
            --virtualsize "${size_bytes}b" \
            subprovisioner

        # deactivate LVM thin LV (`--activate n` has no effect on `lvcreate
        # --type thin`)

        __run_ignoring_error "already exists in volume group" \
            lvm lvchange \
            --devices "$backing_device_path" \
            --activate n \
            "subprovisioner/$lvm_thin_lv_name"
        ;;

    delete)
	    # remove LVM thin LV

        __run_ignoring_error "failed to find logical volume" \
            lvm lvremove \
            --devices "$backing_device_path" \
            "subprovisioner/$lvm_thin_lv_name"

	    # remove LVM thin pool LV if there are no more thin LVs

        __run_ignoring_error "failed to find logical volume|removing pool \S+ will remove" \
            lvm lvremove \
            --devices "$backing_device_path" \
            "subprovisioner/$lvm_thin_pool_lv_name"
        ;;

    activate)
        # activate LVM thin pool LV

        lvm lvchange \
            --devices "$backing_device_path" \
            --activate ey \
            "subprovisioner/$lvm_thin_pool_lv_name"

        # activate LVM thin LV

        lvm lvchange \
            --devices "$backing_device_path" \
            --activate ey \
            "subprovisioner/$lvm_thin_lv_name"
        ;;

    deactivate)
        # deactivate LVM thin LV

        lvm lvchange \
            --devices "$backing_device_path" \
            --activate n \
            "subprovisioner/$lvm_thin_lv_name"

        # deactivate LVM thin pool LV

        lvm lvchange \
            --devices "$backing_device_path" \
            --activate n \
            "subprovisioner/$lvm_thin_pool_lv_name"
        ;;

    snapshot)
        lvm_target_thin_lv_name=$5

        # create snapshot LVM thin LV

        __run_ignoring_error "already exists in volume group" \
            lvm lvcreate \
            --devices "$backing_device_path" \
            --name "$lvm_target_thin_lv_name" \
            --snapshot \
            --setactivationskip n \
            "subprovisioner/$lvm_thin_lv_name"

        # deactivate LVM thin LV (`--activate n` has no effect on `lvcreate
        # --type thin`)

        lvm lvchange \
            --devices "$backing_device_path" \
            --activate n \
            "subprovisioner/$lvm_target_thin_lv_name"
        ;;
esac
