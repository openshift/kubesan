# SPDX-License-Identifier: Apache-2.0

ksan-retry() {
    # shellcheck disable=SC2154
    touch "${kubesan_retry_path}"
    exit 0
}
export -f ksan-retry

# shellcheck disable=SC2154
if (( ! sandbox )); then
    ksan-cancel() {
        # shellcheck disable=SC2154
        touch "${kubesan_cancel_path}"
        exit 0
    }
    export -f ksan-cancel
fi

# Usage: ksan-ssh-into-node <node_name>|<node_index> [<command...>]
ksan-ssh-into-node() {
    if (( $# < 1 )); then
        >&2 echo "Usage: ksan-ssh-into-node <node_name>|<node_index> [<args...>]"
        return 2
    elif (( $# == 1 )); then
        # shellcheck disable=SC2154
        minikube \
            --profile "${current_cluster}" \
            ssh \
            --node "$( __ksan-get-node-name "$1" )" \
            -- \
            bash -i
    else
        local __args="${*:2}"
        minikube \
            --profile "${current_cluster}" \
            ssh \
            --node "$( __ksan-get-node-name "$1" )" \
            -- \
            bash -ic "${__args@Q}" bash
    fi
}
export -f ksan-ssh-into-node
