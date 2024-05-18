# SPDX-License-Identifier: Apache-2.0

# Usage: sp-stage <format> <args...>
sp-stage() {
    (
        set -o errexit -o pipefail -o nounset +o xtrace

        # shellcheck disable=SC2059
        text="$( printf "$@" )"
        text_lower="${text,,}"

        # shellcheck disable=SC2154
        if (( pause_on_stage )); then
            __log_yellow "Pausing before ${text_lower::1}${text:1}"
            __shell 32 false
            if [[ -e "${temp_dir}/retry" ]]; then
                exit 1
            fi
        fi

        printf "\033[36m--- [%6.1f] %s\033[0m\n" "$( __elapsed )" "${text}"
    )
}

# Usage: sp-create-volume <name> <size>
sp-create-volume() {
    name=$1
    size=$2

    sp-stage "Creating volume \"$name\"..."

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

    sp-wait-for-pvc-to-be-bound 300 "$name"
}

# Usage: sp-fill-volume <name> <size_mb>
sp-fill-volume() {
    name=$1
    size_mb=$2

    sp-stage "Writing random data to volume \"$name\"..."

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

    sp-wait-for-pod-to-succeed 60 test-pod
    kubectl delete pod test-pod --timeout=60s
}

# Usage: sp-create-snapshot <volume> <snapshot>
sp-create-snapshot() {
    volume=$1
    snapshot=$2

    sp-stage "Snapshotting \"$volume\"..."

    kubectl create -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: $snapshot
spec:
  volumeSnapshotClassName: subprovisioner
  source:
    persistentVolumeClaimName: $volume
EOF

    sp-wait-for-vs-to-be-bound 60 "$snapshot"
}
