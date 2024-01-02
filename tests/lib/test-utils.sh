# SPDX-License-Identifier: Apache-2.0

# Usage: __stage <format> <args...>
__stage() {
    (
        set -o errexit -o pipefail -o nounset +o xtrace

        # shellcheck disable=SC2059
        text="$( printf "$@" )"
        text_lower="${text,,}"

        # shellcheck disable=SC2154
        if (( pause_on_stage )); then
            __log_yellow "Pausing before ${text_lower::1}${text:1}"
            __debug_shell
        fi

        printf "\033[36m--- [%6.1f] %s\033[0m\n" "$( __elapsed )" "${text}"
    )
}

# Usage: __poll <retry_delay> <max_tries> <command>
__poll() {
    (
        set -o errexit -o pipefail -o nounset +o xtrace

        for (( i = 1; i < "$2"; ++i )); do
            if eval "${*:3}"; then return 0; fi
            sleep "$1"
        done

        if eval "${*:3}"; then return 0; fi

        return 1
    )
}

# Usage: __pod_is_running [-n=<pod_namespace>] <pod_name>
__pod_is_running() {
    [[ "$( kubectl get pod "$@" -o=jsonpath='{.status.phase}' )" = Running ]]
}

# Usage: __wait_for_pod_to_succeed <timeout_seconds> [-n=<pod_namespace>] <pod_name>
__wait_for_pod_to_succeed() {
    __poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Succeeded|Failed$ ]]"
    # shellcheck disable=SC2048,SC2086
    [[ "$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )" = Succeeded ]]
}

# Usage: __wait_for_pod_to_start_running <timeout_seconds> [-n=<pod_namespace>] <pod_name>
__wait_for_pod_to_start_running() {
    __poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Running|Succeeded|Failed$ ]]"
}

# Usage: __wait_for_pvc_to_be_bound <timeout_seconds> [-n=<pvc_namespace>] <pvc_name>
__wait_for_pvc_to_be_bound() {
    __poll 1 "$1" "[[ \"\$( kubectl get pvc ${*:2} -o=jsonpath='{.status.phase}' )\" = Bound ]]"
}

# Usage: __wait_for_vs_to_be_ready <timeout_seconds> [-n=<vs_namespace>] <vs_name>
__wait_for_vs_to_be_ready() {
    __poll 1 "$1" "[[ \"\$( kubectl get vs ${*:2} -o=jsonpath='{.status.readyToUse}' )\" = true ]]"
}
