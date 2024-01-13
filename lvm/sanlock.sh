#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

# TODO: Should we run wdmd too?

host_hostname=$1

command=(
    sanlock
    daemon
    -D
    -w 0
    -U root
    -G root
    -e "$host_hostname"
)

>&2 echo "\$ ${command[*]}"
exec "${command[@]}"
