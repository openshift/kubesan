#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o pipefail -o nounset -o xtrace

command=$1
vol_name=$2

__dmsetup() {
    nsenter -m -u -i -n -p -t 1 dmsetup "$@"
}

# Usage: __run_ignoring_error <error_regex> <cmd...>
__run_ignoring_error() {
    local output
    output=$( "${@:2}" 2>&1 | tee /dev/stderr ) || grep -iE "$1" <<< "$output"
}

# Usage: __lower_table [<connect_to_path>]
__lower_table() {
    if (( $# == 1 )); then
        echo "0 $(( vol_size / 512 )) linear $1 0"
    else
        echo "0 $(( vol_size / 512 )) error"
    fi
}

# Usage: __upper_table
__upper_table() {
    echo "0 $(( vol_size / 512 )) multipath 3 queue_if_no_path queue_mode bio 0 1 1 round-robin 0 1 0 /dev/mapper/$vol_name-lower"
}

case "${command}" in
    create)
        vol_size=$3
        connect_to_path=$4

        __run_ignoring_error "device or resource busy" \
            __dmsetup create "$vol_name-lower" --table "$( __lower_table "$connect_to_path" )"

        __dmsetup mknodes "$vol_name-lower"

        __run_ignoring_error "device or resource busy" \
            __dmsetup create "$vol_name" --table "$( __upper_table )"

        __dmsetup mknodes "$vol_name"
        ;;

    remove)
        # --force replaces table with error target, which prevents hangs when
        # removing a disconnected volume

        __run_ignoring_error "no such device or address" \
            __dmsetup remove "$vol_name" --force

        __run_ignoring_error "no such device or address" \
            __dmsetup remove "$vol_name-lower" --force
        ;;

    connect)
        vol_size=$3
        connect_to_path=$4

        __dmsetup suspend "$vol_name-lower"  # flush any in-flight I/O
        __dmsetup load "$vol_name-lower" --table "$( __lower_table "$connect_to_path" )"
        __dmsetup resume "$vol_name-lower"

        __dmsetup message "$vol_name" 0 "reinstate_path /dev/mapper/$vol_name-lower"
        ;;

    disconnect)
        vol_size=$3

        __dmsetup message "$vol_name" 0 "fail_path /dev/mapper/$vol_name-lower"

        __dmsetup suspend "$vol_name-lower"  # flush any in-flight I/O
        __dmsetup load "$vol_name-lower" --table "$( __lower_table )"
        __dmsetup resume "$vol_name-lower" --noudevsync  # this hangs without --noudevsync
        ;;
esac
