#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o pipefail -o nounset -o xtrace

command=$1
lvm_vg_name=$2
lvm_thin_pool_lv_name=$3

# Usage: __run_ignoring_error <error_regex> <cmd...>
__run_ignoring_error() {
    local output
    output=$( "${@:2}" 2>&1 | tee /dev/stderr ) || grep -iE "$1" <<< "$output"
}

# Run LVM commands outside container because they are designed for a single
# system-wide instance and do not support containers.
__lvm() {
    nsenter --target 1 --all lvm "$@"
}

__lockstart() {
    __lvm vgchange \
        --devicesfile "$lvm_vg_name" \
        --lock-start \
        "$lvm_vg_name"
}

# ensure LVM VG lockspace is started (oftentimes it first fails but works on the
# second try; TODO: figure out why)
__lockstart || __lockstart

case "${command}" in
    lockstart)
        ;;

    activate-pool)
        # activate LVM thin *pool* LV

        __lvm lvchange \
            --devicesfile "$lvm_vg_name" \
            --activate ey \
            "$lvm_vg_name/$lvm_thin_pool_lv_name"
        ;;

    deactivate-pool)
        # deactivate LVM thin *pool* LV

        __lvm lvchange \
            --devicesfile "$lvm_vg_name" \
            --activate n \
            "$lvm_vg_name/$lvm_thin_pool_lv_name"
        ;;

    activate)
        lvm_thin_lv_name=$4

        # activate LVM thin LV

        __lvm lvchange \
            --devicesfile "$lvm_vg_name" \
            --activate ey \
            --monitor y \
            "$lvm_vg_name/$lvm_thin_lv_name"
        ;;

    deactivate)
        lvm_thin_lv_name=$4

        # deactivate LVM thin LV

        __lvm lvchange \
            --devicesfile "$lvm_vg_name" \
            --activate n \
            "$lvm_vg_name/$lvm_thin_lv_name"
        ;;
esac
