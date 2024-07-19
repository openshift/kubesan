#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

# private definitions

# Usage: __elapsed
__elapsed() {
    local delta
    delta=$(( $( date +%s%N ) - start_time ))
    printf '%d.%09d' $(( delta / 10**9 )) $(( delta % 10**9 ))
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
