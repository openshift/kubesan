#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -ex

device_to_export=$1

while [[ ! -b "$device_to_export" ]]; do
    sleep 1
done

exec nbdkit --foreground --port 10809 file "$device_to_export"
