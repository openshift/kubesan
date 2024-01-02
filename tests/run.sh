#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o pipefail -o nounset

start_time="$( date +%s.%N )"
script_dir="$( realpath -e "$0" | xargs dirname )"
repo_root="$( realpath -e "${script_dir}/.." )"

# parse usage

fail_fast=0
num_nodes=2
pause_on_failure=0
pause_on_stage=0
tests=()

while (( $# > 0 )); do
    case "$1" in
        --fail-fast)
            fail_fast=1
            ;;
        --nodes)
            shift
            num_nodes=$1
            ;;
        --pause-on-failure)
            pause_on_failure=1
            ;;
        --pause-on-stage)
            # shellcheck disable=SC2034
            pause_on_stage=1
            ;;
        *)
            tests+=( "$1" )
            ;;
    esac
    shift
done

if (( "${#tests[@]}" == 0 )); then
    >&2 echo -n "\
Usage: $0 [<options...>] <tests...>
       $0 [<options...>] all

Run each given test against a temporary minikube cluster.

If invoked with a single \`all\` argument, all .sh files under t/ are run as
tests.

This actually maintains two minikube clusters, using one to run the current test
while preparing the other in the background, so that the next test has a cluster
ready more quickly. One of the clusters is left running after this script exits,
but will stop itself if this script isn't run again for 30 minutes.

Options:
   --fail-fast          Cancel remaining tests after a test fails.
   --nodes <n>          Number of nodes in the cluster (default: 2).
   --pause-on-failure   Launch an interactive shell after a test fails.
   --pause-on-stage     Launch an interactive shell before each stage in a test.
"
    exit 2
fi

if (( "${#tests[@]}" == 1 )) && [[ "${tests[0]}" = all ]]; then
    tests=()
    for f in "${script_dir}"/t/*.sh; do
        tests+=( "$f" )
    done
fi

for test in "${tests[@]}"; do
    if [[ ! -e "${test}" ]]; then
        >&2 echo "Test file does not exist: ${test}"
        exit 1
    fi
done

# private definitions

# Usage: __elapsed
__elapsed() {
    bc -l <<< "$( date +%s.%N ) - ${start_time}"
}

# Usage: __big_log <color> <format> <args...>
__big_log() {
    local text term_cols sep_len
    text="$( printf "${@:2}" )"
    term_cols="$( tput cols 2> /dev/null )" || term_cols=80
    sep_len="$(( term_cols - ${#text} - 16 ))"
    printf "\033[%sm--- [%6.1f] %s " "$1" "$( __elapsed )" "${text}"
    printf '%*s\033[0m\n' "$(( sep_len < 0 ? 0 : sep_len ))" '' | tr ' ' -
}

# Usage: __log <color> <format> <args...>
__log() {
    # shellcheck disable=SC2059
    printf "\033[%sm--- [%6.1f] %s\033[0m\n" \
        "$1" "$( __elapsed )" "$( printf "${@:2}" )"
}

# Usage: __log_red <format> <args...>
__log_red() {
    __log 31 "$@"
}

# Usage: __log_yellow <format> <args...>
__log_yellow() {
    __log 33 "$@"
}

# Usage: __log_cyan <format> <args...>
__log_cyan() {
    __log 36 "$@"
}

__debug_shell() {
    # shellcheck disable=SC2016
    __log_red 'Starting interactive shell for debugging.'
    __log_red 'Inspect the cluster with:'
    __log_red '  $ kubectl [...]'
    __log_red '  $ __controller_plugin describe|exec|logs [<args...>]'
    __log_red '  $ __node_plugin <node_name>|<node_index> describe|exec|logs [<args...>]'
    __log_red '  $ __ssh_into_node <node_name>|<node_index> [<command...>]'
    ( cd "${temp_dir}" && "${BASH}" ) || true
}

# Usage: __failure <format> <args...>
__failure() {
    __log_red "$@"

    if (( pause_on_failure )); then
        __debug_shell
    fi
}

# definitions shared with test scripts

export REPO_ROOT=${repo_root}
export TEST_IMAGE=docker.io/localhost/clustered-csi/test:test

for f in debug-utils.sh test-utils.sh; do
    # shellcheck disable=SC1090
    source "${script_dir}/lib/$f"
done

# build images

__log_cyan "Building clustered-csi image (localhost/clustered-csi/clustered-csi:test)..."
podman image build -t localhost/clustered-csi/clustered-csi:test "${repo_root}"

__log_cyan "Building test image (localhost/clustered-csi/test:test)..."
podman image build -t localhost/clustered-csi/test:test "${script_dir}/lib/test-image"

# create temporary directory

temp_dir="$( mktemp -d )"
trap 'rm -fr "${temp_dir}"' EXIT

# run tests

test_i=0
num_succeeded=0
num_failed=0

canceled=0
trap 'canceled=1' SIGINT

__minikube() {
    minikube --profile="${current_cluster}" "$@"
}

# Usage: __minikube_ssh <node> <command...>
__minikube_ssh() {
    __minikube ssh --node="$1" -- "
        set -o errexit -o pipefail -o nounset
        source .bashrc
        ${*:2}
        "
}

# Usage: __minikube_cluster_exists <suffix>
__minikube_cluster_exists() {
    local __exit_code=0
    minikube --profile "$1" status &>/dev/null || __exit_code="$?"

    case "${__exit_code}" in
    0|1|2|3|4|5|6|7)
        return 0
        ;;
    85)
        return 1
        ;;
    *)
        >&2 echo "minikube failed with exit code ${__exit_code}"
        exit "${__exit_code}"
        ;;
    esac
}

# Usage: __restart_minikube_cluster <profile> [<extra_minikube_opts...>]
__restart_minikube_cluster() {
    minikube start \
        --profile="$1" \
        --driver=kvm2 \
        --keep-context \
        "${@:2}"
}

# Usage: __start_minikube_cluster <profile> [<extra_minikube_opts...>]
__start_minikube_cluster() {
    __restart_minikube_cluster "$@" --nodes="${num_nodes}"
}

# Usage: __create_minikube_cluster_async <profile> [<extra_minikube_opts...>]
__create_minikube_cluster_async() {
    __log_cyan "Creating minikube cluster '%s' in the background to use later..." "$1"
    creating_cluster_in_background=1
    __start_minikube_cluster "$@" &>/dev/null &
}

cluster_base_name=$( printf 'clustered-csi-test-%dn' "${num_nodes}" )

__next_cluster() {
    case "$1" in
    clustered-csi-test-*n-a)
        echo "${cluster_base_name}-b"
        ;;
    clustered-csi-test-*n-b)
        echo "${cluster_base_name}-a"
        ;;
    *)
        exit 1
        ;;
    esac
}

export current_cluster

for test in "${tests[@]}"; do

    unset KUBECONFIG

    test_name="$( realpath --relative-to=. "${test}" )"
    test_resolved="$( realpath -e "${test}" )"

    __big_log 33 'Running test %s (%d of %d)...' \
        "${test_name}" "$(( ++test_i ))" "${#tests[@]}"

    __log_cyan "Starting NBD server to serve as a shared block device..."

    rm -f "${temp_dir}/backing.raw"
    truncate -s 1G "${temp_dir}/backing.raw"

    nbd-server \
        --pid-file "${temp_dir}/nbd-server.pid" \
        10809 \
        "${temp_dir}/backing.raw"
    nbd_server_pid=$( cat "${temp_dir}/nbd-server.pid" )

    trap '{
        kill "${nbd_server_pid}" && tail --pid="${nbd_server_pid}" -f /dev/null
        rm -fr "${temp_dir}"
        }' EXIT

    if ! __minikube_cluster_exists "${cluster_base_name}-a" &&
        ! __minikube_cluster_exists "${cluster_base_name}-b"; then

        current_cluster="${cluster_base_name}-a"
        background_cluster="$( __next_cluster "${current_cluster}" )"

        __create_minikube_cluster_async "${background_cluster}"

        __log_cyan "Creating and using minikube cluster '%s'..." "${current_cluster}"
        __start_minikube_cluster "${current_cluster}"

    else

        if [[ -n "${current_cluster:-}" ]]; then
            if kill -0 "$!" &>/dev/null; then
                __log_cyan "Waiting for minikube cluster '%s' to be ready..." "${background_cluster}"
            fi
            wait || true
            current_cluster="${background_cluster}"
        elif __minikube_cluster_exists "${cluster_base_name}-a"; then
            current_cluster="${cluster_base_name}-a"
        else
            current_cluster="${cluster_base_name}-b"
        fi

        background_cluster="$( __next_cluster "${current_cluster}" )"

        if ! __minikube_cluster_exists "${background_cluster}"; then
            __create_minikube_cluster_async "${background_cluster}"
        fi

        __log_cyan "Using existing minikube cluster '%s'..." "${current_cluster}"
        __minikube stop --keep-context-active --cancel-scheduled
        if [[ "$( __minikube status --format='{{.Host}}' )" != Running ]]; then
            __restart_minikube_cluster "${current_cluster}"
        fi
    fi

    trap '{
        __minikube delete
        kill "${nbd_server_pid}" && tail --pid="${nbd_server_pid}" -f /dev/null
        rm -fr "${temp_dir}"
        wait
        }' EXIT

    kubectl config view > "${temp_dir}/kubeconfig"
    export KUBECONFIG="${temp_dir}/kubeconfig"
    kubectl config use-context "${current_cluster}"

    export NODES=( "${current_cluster}" )
    for (( i = 2; i <= num_nodes; ++i )); do
        NODES+=( "$( printf '%s-m%02d' "${current_cluster}" "$i" )" )
    done
    export NODE_INDICES=( "${!NODES[@]}" )

    __log_cyan "Importing clustered-csi images into minikube cluster '%s'..." "${current_cluster}"
    for image in clustered-csi test; do
        podman save "clustered-csi/${image}:test" | __minikube image load -
    done

    for node in "${NODES[@]}"; do
        __minikube cp \
            "${script_dir}/lib/node-bash-profile.sh" \
            "${node}:/home/docker/.bashrc"
    done

    set +o errexit
    (
        set -o errexit -o pipefail -o nounset +o xtrace

        __log_cyan "Attaching shared block device to all minikube nodes..."

        for node in "${NODES[@]}"; do
            __minikube_ssh "${node}" "
                sudo modprobe nbd nbds_max=1
                __run_in_test_container --net host -- \
                    nbd-client host.minikube.internal /dev/nbd0
                "
        done

        __log_cyan "Starting lvmlockd and sanlock daemons on all nodes..."

        for i in "${!NODES[@]}"; do
            __minikube_ssh "${NODES[i]}" "
                sudo sed -i -E 's/#? ?use_lvmlockd = [01]/use_lvmlockd = 1/' /etc/lvm/lvm.conf
                sudo sed -i -E 's/#? ?udev_sync = [01]/udev_sync = 0/' /etc/lvm/lvm.conf
                sudo sed -i -E 's/#? ?udev_rules = [01]/udev_rules = 0/' /etc/lvm/lvm.conf
                sudo mkdir -p /run/lvm
                __run_in_test_container_async \
                    --name lvmlockd -- \
                    lvmlockd --daemon-debug --gl-type sanlock --host-id $((i+1))
                __run_in_test_container_async \
                    --name sanlock --pid container:lvmlockd -- \
                    sanlock daemon -D -w 0 -U root -G root -e \$( hostname )
                # TODO: Should we run wdmd too?
                "
        done

        __log_cyan "Creating LVM volume group..."

        __minikube_ssh "${NODES[0]}" "
            __run_in_test_container vgcreate --lock-type sanlock \
                clustered-csi-vg /dev/nbd0
            "

        __log_cyan "Starting LVM volume group on all nodes..."

        for node in "${NODES[@]}"; do
            __minikube_ssh "${node}" "
                # for some reason, trying a second time seems to resolve failures
                __run_in_test_container vgchange --lock-start clustered-csi-vg || {
                    __run_in_test_container vgchange --lock-start clustered-csi-vg
                }
                "
        done
    )
    exit_code="$?"
    set -o errexit

    if (( exit_code != 0 )); then
        __failure 'Failed to set up LVM volume group.'
    else
        set +o errexit
        (
            set -o errexit -o pipefail -o nounset +o xtrace

            __log_cyan "Installing clustered-csi..."
            sed -E 's|quay.io/clustered-csi/([a-z-]+):[0-9+\.]+|docker.io/localhost/clustered-csi/\1:test|g' \
                "${repo_root}/deployment.yaml" | kubectl create -f -

            __log_cyan "Creating common objects..."
            kubectl create -f "${script_dir}/lib/common-objects.yaml"

            set -o xtrace
            cd "$( dirname "${test_resolved}" )"
            # shellcheck disable=SC1090
            source "${test_resolved}"
        )
        exit_code="$?"
        set -o errexit

        if (( exit_code == 0 )); then

            __log_cyan "Uninstalling clustered-csi..."
            kubectl delete --ignore-not-found --timeout=30s \
                -f "${repo_root}/deployment.yaml" \
                || exit_code="$?"

            if (( exit_code != 0 )); then
                __failure 'Failed to uninstall clustered-csi.'
            fi

        else

            __failure 'Test %s failed.' "${test_name}"

        fi
    fi

    __log_cyan "Deleting minikube cluster '%s'..." "${current_cluster}"
    __minikube delete

    __log_cyan "Stopping NBD server..."
    kill "${nbd_server_pid}" && tail --pid="${nbd_server_pid}" -f /dev/null

    trap '{
        rm -fr "${temp_dir}"
        wait
        }' EXIT

    if (( canceled )); then
        break
    elif (( exit_code == 0 )); then
        : $(( num_succeeded++ ))
    else
        : $(( num_failed++ ))
        if (( fail_fast )); then
            break
        fi
    fi

done

# print summary

num_canceled="$(( ${#tests[@]} - num_succeeded - num_failed ))"

if (( num_failed > 0 )); then
    color=31
elif (( num_canceled > 0 )); then
    color=33
else
    color=32
fi

__big_log "${color}" '%d succeeded, %d failed, %d canceled' \
    "${num_succeeded}" "${num_failed}" "${num_canceled}"

if (( "${creating_cluster_in_background:-0}" == 1 )) && kill -0 "$!" &>/dev/null; then
    __log_cyan "Waiting for minikube cluster '%s' creation to finish before terminating..." \
        "${background_cluster}"
    wait || true
    __log_cyan "Done."
else
    wait || true
fi

minikube stop \
    --profile="${background_cluster}" \
    --keep-context-active \
    --schedule=30m

(( num_succeeded == ${#tests[@]} ))
