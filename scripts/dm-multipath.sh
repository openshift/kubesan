#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o pipefail -o nounset -o xtrace

command=$1
vol_name=$2

# Usage: __run_ignoring_error <error_regex> <cmd...>
__run_ignoring_error() {
    local output
    output=$( "${@:2}" 2>&1 | tee /dev/stderr ) || grep -iE "$1" <<< "$output"
}

# Usage: __table <size> [<connect_to_path>]
__table() {
    # dm-thin is bio-based, so dm-multipath has to be bio-based as well, hence
    # the "queue_mode bio" option.
    echo -n "0 $(( $1 / 512 )) multipath 3 queue_if_no_path queue_mode bio 0"
    if (( $# == 1 )); then
        echo " 0 0"
    else
        echo " 1 1 round-robin 0 1 0 $2"
    fi
}

case "${command}" in
    create)
        vol_size=$3
        connect_to_path=$4

        # TODO: make idempotent
        dmsetup create "$vol_name" --table "$( __table "$vol_size" "$connect_to_path" )"
        ;;

    remove)
        # --force replaces table with error target, which prevents hangs when
        # removing a disconnected volume
        __run_ignoring_error "no such device or address" \
            dmsetup remove "$vol_name" --force
        ;;

    connect)
        vol_size=$3
        connect_to_path=$4

        dmsetup load "$vol_name" --table "$( __table "$vol_size" "$connect_to_path" )"
        dmsetup resume "$vol_name"
        ;;

    disconnect)
        vol_size=$3

        dmsetup load "$vol_name" --table "$( __table "$vol_size" )"
        dmsetup resume "$vol_name" --noudevsync  # this hangs without --noudevsync
        ;;
esac
