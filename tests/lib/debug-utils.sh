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
# Only use for interactive debugging, do not assume node ssh access is
# available in tests because that would require the user to provide ssh
# connection details and that is inconvenient.
ksan-ssh-into-node() {
    if (( $# < 1 )); then
        >&2 echo "Usage: ksan-ssh-into-node <node_name>|<node_index> [<args...>]"
        return 2
    fi
    ksan-${deploy_tool}-ssh-into-node "$@"
}
export -f ksan-ssh-into-node
