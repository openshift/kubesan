#!/bin/bash
# Usage: ./nbdexport-helper.sh uri orig
#
# Helper script for tests/t/nbdexport.sh. The second node in the cluster
# runs this script with the URI to the NBD export exposed by the first
# node, and checks that it appears to be the same image as the ORIG drive.

echo "starting $0, arguments: $@"

set -e
test $# = 2
test -b "$2"

uri=$1
orig=$2

# TODO Update to nbds: when TLS support added
[[ $uri =~ nbd://[-.a-z0-9]+/[-a-z0-9]+ ]]

# qemu-io should report the same size
expect=$(qemu-io -f raw -c length "${orig}")
actual=$(qemu-io -f raw -c length "${uri}")
[[ $expect == $actual ]]

# qemu should see the same contents in the first 4k. Note that the
# unit test framework has already activated $orig as a shared VG,
# so writing to the image from here is unsafe, and reads may spot
# spurious differences in the portion of the image used by sanlock
# lease updates - but the first 4k should be stable.
expect=$(qemu-io -f raw -c 'r -v 0 4k' "${orig}" | sed '/^read/,$D')
actual=$(qemu-io -f raw -c 'r -v 0 4k' "${uri}" | sed '/^read/,$D')
[[ $expect == $actual ]]

echo "Comparison passed"
