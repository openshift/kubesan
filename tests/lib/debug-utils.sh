# SPDX-License-Identifier: Apache-2.0

sp-retry() {
    # shellcheck disable=SC2154
    touch "${subprovisioner_retry_path}"
    exit 0
}
export -f sp-retry

# Usage: sp-ssh-into-node <node_name>|<node_index> [<command...>]
sp-ssh-into-node() {
    if (( $# < 1 )); then
        >&2 echo "Usage: sp-ssh-into-node <node_name>|<node_index> [<args...>]"
        return 2
    elif (( $# == 1 )); then
        # shellcheck disable=SC2154
        minikube \
            --profile "${current_cluster}" \
            ssh \
            --node "$( __sp-get-node-name "$1" )" \
            -- \
            bash -i
    else
        local __args="${*:2}"
        minikube \
            --profile "${current_cluster}" \
            ssh \
            --node "$( __sp-get-node-name "$1" )" \
            -- \
            bash -ic "${__args@Q}" bash
    fi
}
export -f sp-ssh-into-node
