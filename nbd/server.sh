#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -ex

# TODO: Probably want to use nbdkit instead.

device_to_export=$1

while [[ ! -b "$device_to_export" ]]; do
    sleep 1
done

exec nbd-server 10809 "$device_to_export" --nodaemon
