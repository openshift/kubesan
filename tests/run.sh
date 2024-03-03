#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o pipefail -o nounset

if [[ -n "${subprovisioner_tests_run_sh_path:-}" ]]; then
    >&2 echo "You're already running $0"
    exit 1
fi

start_time="$( date +%s.%N )"
script_dir="$( realpath -e "$0" | xargs dirname )"
repo_root="$( realpath -e "${script_dir}/.." )"
initial_working_dir="$( pwd )"

# parse usage

fail_fast=0
num_nodes=2
repeat=1
set_kubectl_context=0
pause_on_failure=0
pause_on_stage=0
tests_arg=()

while (( $# > 0 )); do
    case "$1" in
        --fail-fast)
            fail_fast=1
            ;;
        --nodes)
            shift
            num_nodes=$1
            ;;
        --repeat)
            shift
            repeat=$1
            ;;
        --set-kubectl-context)
            set_kubectl_context=1
            ;;
        --pause-on-failure)
            pause_on_failure=1
            ;;
        --pause-on-stage)
            # shellcheck disable=SC2034
            pause_on_stage=1
            ;;
        *)
            tests_arg+=( "$1" )
            ;;
    esac
    shift
done

if (( "${#tests_arg[@]}" == 0 )); then
    >&2 echo -n "\
Usage: $0 [<options...>] <tests...>
       $0 [<options...>] all
       $0 [<options...>] sandbox

Run each given test against a temporary minikube cluster.

If invoked with a single \`all\` argument, all .sh files under t/ are run as
tests.

If invoked with a single \`sandbox\` argument, no tests are actually run but a
cluster is set up and an interactive shell is launched so you can play around
with it.

This actually maintains two minikube clusters, using one to run the current test
while preparing the other in the background, so that the next test has a cluster
ready more quickly. One of the clusters is left running after this script exits,
but will stop itself if this script isn't run again for 30 minutes.

Options:
   --fail-fast             Cancel remaining tests after a test fails.
   --nodes <n>             Number of nodes in the cluster (default: 2).
   --repeat <n>            Run each test n times (default: 1).
   --set-kubectl-context   Update the current user's kubectl context to point at the cluster.
   --pause-on-failure      Launch an interactive shell after a test fails.
   --pause-on-stage        Launch an interactive shell before each stage in a test.
"
    exit 2
fi

if (( "${#tests_arg[@]}" == 1 )) && [[ "${tests_arg[0]}" = sandbox ]]; then
    sandbox=1
else
    sandbox=0

    if (( "${#tests_arg[@]}" == 1 )) && [[ "${tests_arg[0]}" = all ]]; then
        tests_arg=()
        for f in "${script_dir}"/t/*.sh; do
            tests_arg+=( "$f" )
        done
    fi

    for test in "${tests_arg[@]}"; do
        if [[ ! -e "${test}" ]]; then
            >&2 echo "Test file does not exist: ${test}"
            exit 1
        fi
    done
fi

tests=()
for test in "${tests_arg[@]}"; do
    for (( i = 0; i < repeat; ++i )); do
        tests+=( "$test" )
    done
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

# Usage: __log_green <format> <args...>
__log_green() {
    __log 32 "$@"
}

# Usage: __log_yellow <format> <args...>
__log_yellow() {
    __log 33 "$@"
}

# Usage: __log_cyan <format> <args...>
__log_cyan() {
    __log 36 "$@"
}

__shell() {
    __log "$1" 'Starting interactive shell.'
    __log "$1" 'Inspect the cluster with:'
    __log "$1" '  $ kubectl [...]'
    __log "$1" '  $ __controller_plugin describe|exec|logs [<args...>]'
    __log "$1" '  $ __node_plugin <node_name>|<node_index> describe|exec|logs [<args...>]'
    __log "$1" '  $ __lvmlockd <node_name>|<node_index> describe|exec|logs [<args...>]'
    __log "$1" '  $ __sanlock <node_name>|<node_index> describe|exec|logs [<args...>]'
    __log "$1" '  $ __ssh_into_node <node_name>|<node_index> [<command...>]'

    if [[ "$2" != true ]]; then
        __log "$1" 'To retry the current test:'
        __log "$1" '  $ retry'
    fi

    IFS='/' read -r -a script_path <<< "$0"

    if [[ "${script_path[0]}" == "" ]]; then
        # absolute path
        script_path=( "/${script_path[1]}" "${script_path[@]:2}" )
    fi

    if (( ${#script_path[@]} > 2 )); then
        script_path=( ... "${script_path[@]: -2}" )
    fi

    subprovisioner_tests_run_sh_path=$( printf '/%s' "${script_path[@]}" )
    subprovisioner_tests_run_sh_path=${subprovisioner_tests_run_sh_path:1}

    (
        export subprovisioner_tests_run_sh_path
        export subprovisioner_retry_path="${temp_dir}/retry"
        cd "${initial_working_dir}"
        # shellcheck disable=SC2016,SC2028
        "$BASH" --init-file <( echo "
            . \"\$HOME/.bashrc\"
            PROMPT_COMMAND=(
                \"echo -en '\\001\\033[1m\\002(\$subprovisioner_tests_run_sh_path)\\001\\033[0m\\002 '\"
                \"\${PROMPT_COMMAND[@]}\"
                )
            " )
    ) || true
}

# Usage: __failure <format> <args...>
__failure() {
    __log_red "$@"

    if (( pause_on_failure )); then
        __shell 31 false
    fi
}

# Usage: __canceled <format> <args...>
__canceled() {
    __log_yellow "$@"

    if (( pause_on_failure )); then
        __shell 33 false
    fi
}

# definitions shared with test scripts

export REPO_ROOT=${repo_root}
export TEST_IMAGE=docker.io/localhost/subprovisioner/test:test

for f in debug-utils.sh test-utils.sh; do
    # shellcheck disable=SC1090
    source "${script_dir}/lib/$f"
done

# build images

__build_images() {
    __log_cyan "Building Subprovisioner image (localhost/subprovisioner/subprovisioner:test)..."
    podman image build -t localhost/subprovisioner/subprovisioner:test "${repo_root}"

    __log_cyan "Building test image (localhost/subprovisioner/test:test)..."
    podman image build -t localhost/subprovisioner/test:test "${script_dir}/lib/test-image"
}

__build_images

# create temporary directory

temp_dir="$( mktemp -d )"
trap 'rm -fr "${temp_dir}"' EXIT

# run tests

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
        --cpus=2 \
        --memory=2g \
        --disk-size=5g \
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

__wait_until_background_cluster_is_ready() {
    if (( "${creating_cluster_in_background:-0}" == 1 )) && kill -0 "$!" &>/dev/null; then
        __log_cyan "Waiting for minikube cluster '%s' creation to finish before terminating..." \
            "${background_cluster}"
        wait || true
        __log_cyan "Done."
    else
        wait || true
    fi
}

cluster_base_name=$( printf 'subprovisioner-test-%dn' "${num_nodes}" )

__next_cluster() {
    case "$1" in
    subprovisioner-test-*n-a)
        echo "${cluster_base_name}-b"
        ;;
    subprovisioner-test-*n-b)
        echo "${cluster_base_name}-a"
        ;;
    *)
        exit 1
        ;;
    esac
}

export current_cluster

maintain_two_clusters=$(( num_nodes <= 2 ))

__run() {

    if [[ -e "${temp_dir}/retry" ]]; then
        rm -f "${temp_dir}/retry"
        __build_images
    fi

    trap 'rm -fr "${temp_dir}"' EXIT

    unset KUBECONFIG

    if (( maintain_two_clusters )); then

        if ! __minikube_cluster_exists "${cluster_base_name}-a" &&
            ! __minikube_cluster_exists "${cluster_base_name}-b"; then

            current_cluster="${cluster_base_name}-a"

            background_cluster="$( __next_cluster "${current_cluster}" )"
            __create_minikube_cluster_async "${background_cluster}"

            __log_cyan "Creating and using minikube cluster '%s'..." "${current_cluster}"
            __start_minikube_cluster "${current_cluster}"

        else

            if [[ -n "${current_cluster:-}" && -n "${!:-}" ]]; then
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

    else

        current_cluster="${cluster_base_name}"

        if ! __minikube_cluster_exists "${current_cluster}"; then


            __log_cyan "Creating and using minikube cluster '%s'..." "${current_cluster}"
            __start_minikube_cluster "${current_cluster}"

        else

            __log_cyan "Using existing minikube cluster '%s'..." "${current_cluster}"
            __minikube stop --keep-context-active --cancel-scheduled
            if [[ "$( __minikube status --format='{{.Host}}' )" != Running ]]; then
                __restart_minikube_cluster "${current_cluster}"
            fi
        fi

    fi

    trap '{
        __minikube delete
        rm -fr "${temp_dir}"
        __wait_until_background_cluster_is_ready
        }' EXIT

    if (( set_kubectl_context )); then
        kubectl config use-context "${current_cluster}"
    fi

    kubectl config view > "${temp_dir}/kubeconfig"
    export KUBECONFIG="${temp_dir}/kubeconfig"
    kubectl config use-context "${current_cluster}"

    export NODES=( "${current_cluster}" )
    for (( i = 2; i <= num_nodes; ++i )); do
        NODES+=( "$( printf '%s-m%02d' "${current_cluster}" "$i" )" )
    done

    export NODE_INDICES=( "${!NODES[@]}" )

    export NODE_IPS=()
    for node in "${NODES[@]}"; do
        NODE_IPS+=( "$( __minikube ip --node="${node}" )" )
    done

    __log_cyan "Importing Subprovisioner images into minikube cluster '%s'..." "${current_cluster}"
    for image in subprovisioner test; do
        podman save "subprovisioner/${image}:test" | __minikube image load -
    done

    for node in "${NODES[@]}"; do
        __minikube cp \
            "${script_dir}/lib/node-bash-profile.sh" \
            "${node}:/home/docker/.bashrc"
    done

    __log_cyan "Starting NBD server to serve as a shared block device..."

    __minikube_ssh "${NODES[0]}" "
        sudo truncate -s 4G /mnt/vda1/backing.raw
        __run_in_test_container_async --net host -v /mnt/vda1/backing.raw:/disk -- \
            nbd-server --nodaemon --config-file /dev/null 10809 /disk
        __run_in_test_container --net host -- bash -c '
            for (( i = 0; i < 50; ++i )); do
                if nc -z localhost 10809; then exit 0; fi
                sleep 0.1
            done
            exit 1
            '
        "

    __log_cyan "Attaching shared block device to all cluster nodes..."

    for node in "${NODES[@]}"; do
        __minikube_ssh "${node}" "
            sudo modprobe nbd nbds_max=16  # for Subprovisioner to use as well
            __run_in_test_container --net host -- \
                nbd-client ${NODE_IPS[0]} /dev/nbd0
            sudo ln -s /dev/nbd0 /dev/subprovisioner-backing-volume
            sudo cp -r /dev/nbd0 /dev/my-san-lun  # good for demos
            "
    done

    set +o errexit
    (
        set -o errexit -o pipefail -o nounset +o xtrace

        __log_cyan "Installing Subprovisioner..."
        for file in "${repo_root}/deploy/0"*; do
            sed \
                -E 's|quay.io/subprovisioner/([a-z-]+):[0-9+\.]+|docker.io/localhost/subprovisioner/\1:test|g' \
                "$file" \
                | kubectl create -f -
        done

        __log_cyan "Enabling volume snapshot support in the cluster..."
        base_url=https://github.com/kubernetes-csi/external-snapshotter
        kubectl create -k "${base_url}/client/config/crd?ref=v7.0.1"
        kubectl create -k "${base_url}/deploy/kubernetes/snapshot-controller?ref=v7.0.1"
        unset base_url

        __log_cyan "Creating common objects..."
        kubectl patch sc standard \
            -p '{ "metadata": { "annotations": { "storageclass.kubernetes.io/is-default-class": "false" } } }'
        kubectl create -f "${script_dir}/lib/common-objects.yaml"

        if (( sandbox )); then
            __shell 32 true
        else
            set -o xtrace
            cd "$( dirname "${test_resolved}" )"
            # shellcheck disable=SC1090
            source "${test_resolved}"
        fi
    )
    exit_code="$?"
    set -o errexit

    if (( exit_code == 0 )); then

        if ! (( sandbox )); then
            __log_cyan "Uninstalling Subprovisioner..."
            kubectl delete --ignore-not-found --timeout=60s \
                -k "${repo_root}/deploy" \
                || exit_code="$?"

            if (( exit_code != 0 )); then
                __failure 'Failed to uninstall Subprovisioner.'
            fi
        fi

    else

        if (( canceled )); then
            echo
            __canceled 'Test %s was canceled.' "${test_name}"
        else
            __failure 'Test %s failed.' "${test_name}"
        fi

    fi

    __log_cyan "Deleting minikube cluster '%s'..." "${current_cluster}"
    __minikube delete

    trap '{
        rm -fr "${temp_dir}"
        __wait_until_background_cluster_is_ready
        }' EXIT
}

if (( sandbox )); then
    __big_log 33 'Starting sandbox cluster...'
    __run
    while [[ -e "${temp_dir}/retry" ]]; do
        __run
    done
else
    for (( test_i = 0; test_i < ${#tests[@]}; ++test_i )); do

        test="${tests[test_i]}"
        test_name="$( realpath --relative-to=. "${test}" )"
        test_resolved="$( realpath -e "${test}" )"

        __big_log 33 'Running test %s (%d of %d)...' \
            "${test_name}" "$(( test_i+1 ))" "${#tests[@]}"

        __run

        if [[ -e "${temp_dir}/retry" ]]; then
            canceled=0
            : $(( --test_i ))
        elif (( canceled )); then
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
fi

trap 'rm -fr "${temp_dir}"' EXIT
__wait_until_background_cluster_is_ready

if [[ -n "${background_cluster:-}" ]]; then
    minikube stop \
        --profile="${background_cluster}" \
        --keep-context-active \
        --schedule=30m
fi

(( sandbox || num_succeeded == ${#tests[@]} ))
