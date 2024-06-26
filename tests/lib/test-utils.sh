# SPDX-License-Identifier: Apache-2.0

# Usage: ksan-stage <format> <args...>
ksan-stage() {
    (
        set -o errexit -o pipefail -o nounset +o xtrace

        # shellcheck disable=SC2059
        text="$( printf "$@" )"
        text_lower="${text,,}"

        # shellcheck disable=SC2154
        if (( pause_on_stage )); then
            __log_yellow "Pausing before ${text_lower::1}${text:1}"
            __shell 32 false
            if [[ -e "${temp_dir}/retry" || -e "${temp_dir}/cancel" ]]; then
                exit 1
            fi
        fi

        printf "\033[36m--- [%6.1f] %s\033[0m\n" "$( __elapsed )" "${text}"
    )
}

# Usage: ksan-create-volume <name> <size>
ksan-create-volume() {
    name=$1
    size=$2

    ksan-stage "Creating volume \"$name\"..."

    kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: $name
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: $size
  volumeMode: Block
EOF

    ksan-wait-for-pvc-to-be-bound 300 "$name"
}

# Usage: ksan-create-fs-volume <name> <size>
ksan-create-fs-volume() {
    name=$1
    size=$2

    ksan-stage "Creating Filesystem volume \"$name\"..."

    kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: $name
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: $size
  volumeMode: Filesystem
EOF

    ksan-wait-for-pvc-to-be-bound 300 "$name"
}

# Usage: ksan-fill-volume <name> <size_mb>
ksan-fill-volume() {
    name=$1
    size_mb=$2

    ksan-stage "Writing random data to volume \"$name\"..."

    kubectl create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  restartPolicy: Never
  containers:
    - name: container
      image: $TEST_IMAGE
      command:
        - bash
        - -c
        - |
          set -o errexit -o pipefail -o nounset -o xtrace
          dd if=/dev/urandom of=/var/pvc conv=fsync bs=1M count=$size_mb
      volumeDevices:
        - { name: $name, devicePath: /var/pvc }
  volumes:
    - { name: $name, persistentVolumeClaim: { claimName: $name } }
EOF

    ksan-wait-for-pod-to-succeed 60 test-pod
    kubectl delete pod test-pod --timeout=60s
}

# Usage: ksan-create-snapshot <volume> <snapshot>
ksan-create-snapshot() {
    volume=$1
    snapshot=$2

    ksan-stage "Snapshotting \"$volume\"..."

    kubectl create -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: $snapshot
spec:
  volumeSnapshotClassName: kubesan
  source:
    persistentVolumeClaimName: $volume
EOF

    ksan-wait-for-vs-to-be-bound 60 "$snapshot"
}
