#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -ex

device_symlink=$1

exec nbd-client -d "$( readlink -e "$device_symlink" )"
