#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

# TODO: Probably want to use nbdkit instead.

device_to_export=$1

command=(
    nbd-server
    10809
    "$device_to_export"
    --nodaemon
)

>&2 echo "\$ ${command[*]}"
exec "${command[@]}"
