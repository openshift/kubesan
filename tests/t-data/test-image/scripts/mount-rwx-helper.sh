#!/bin/bash
# Usage: ./mount-rwx-helper.sh id num_nodes
#
# Helper script for tests/t/mount-rwx.sh. Each node in the cluster
# runs this script with visibility to two RWX PVCs; the script writes
# to offsets determined by id, then reads from the next offset (modulo
# num_nodes) until it can see the expected data from the adjacent
# node.  Reads and writes must be done with O_DIRECT to ensure that
# data makes it to the underlying volume, rather than merely the
# kernel page cache of the container running the script.

echo "starting $0, arguments: $@"

set -e
test $# = 2
test -b /var/pvc1
test -b /var/pvc2

id=$1
num_nodes=$2
test "${id}" -lt "${num_nodes}"
next=$(( ( ${id} + 1 ) % $2 ))

# Data chunks are exactly 64k, to maximize ease of use with O_DIRECT
printf "% $(( 64*1024 ))d" ${id} > /tmp/local_data
printf "% $(( 64*1024 ))d" ${next} > /tmp/next_data

echo "Storing ${id} into /var/pvc1"
dd if=/tmp/local_data of=/var/pvc1 bs=64k seek=${id} count=1 oflag=direct
echo "Storing ${next} into /var/pvc2"
dd if=/tmp/next_data of=/var/pvc2 bs=64k seek=${next} count=1 oflag=direct

# Note: this part has no local time limits because we cannot control
# how long it takes for the neighbor pod to start executing.  But this
# should run to completion fairly quickly once all the pods have
# started, and mount-rwx.sh enforces an overall time limit.
echo "Waiting for expected data to appear in /var/pvc[12]"
counter=0
while :; do
    if dd if=/var/pvc1 bs=64k skip=${next} count=1 iflag=direct |
           cmp /tmp/next_data - &&
       dd if=/var/pvc2 bs=64k skip=${id} count=1 iflag=direct |
           cmp /tmp/local_data - ; then
        echo "All expected data found, exiting"
        exit 0
    fi
    echo "No match yet after ${counter} seconds, sleeping"
    sleep 1
    counter=$(( ${counter} + 1 ))
done
