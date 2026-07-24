#!/usr/bin/env bash

NATIVE_VALIDATION_BASE=""
NATIVE_WORK_ROOT=""

flashgate_require_real_directory() {
    local path="$1"

    [[ -d "$path" && ! -L "$path" ]] || {
        printf 'real directory required: %s\n' "$path" >&2
        return 1
    }
}

flashgate_validate_run_id() {
    local value="$1"

    [[ -n "$value" ]] || return 1
    [[ "$value" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$ ]] || return 1
    [[ "$value" != "." && "$value" != ".." && "$value" != .* ]] || return 1
    [[ "$value" != *"/"* && "$value" != *"\\"* ]] || return 1
    [[ "$value" != *[$' \t\r\n']* ]] || return 1
}

safe_remove_work_root() {
    local resolved_home
    local expected

    flashgate_validate_run_id "${FG_RUN_ID:-}" || {
        printf 'refusing cleanup because FG_RUN_ID is invalid\n' >&2
        return 1
    }
    [[ -n "$NATIVE_VALIDATION_BASE" && -n "$NATIVE_WORK_ROOT" ]] || {
        printf 'refusing cleanup because native paths are unresolved\n' >&2
        return 1
    }
    flashgate_require_real_directory "$NATIVE_VALIDATION_BASE" || return 1
    resolved_home="$(realpath -e -- "$HOME")" || return 1
    expected="$NATIVE_VALIDATION_BASE/$FG_RUN_ID"
    [[ "$NATIVE_WORK_ROOT" == "$expected" ]] || {
        printf 'refusing cleanup outside the direct-child work root\n' >&2
        return 1
    }
    case "$NATIVE_WORK_ROOT" in
        ""|/|/home|"$resolved_home"|"$NATIVE_VALIDATION_BASE")
            printf 'refusing cleanup of a protected directory\n' >&2
            return 1
            ;;
    esac

    if ! python3 "$NATIVE_SAFETY_HELPER" remove \
        --base "$NATIVE_VALIDATION_BASE" \
        --run-id "$FG_RUN_ID"; then
        printf 'safe work-root cleanup failed closed\n' >&2
        trap - EXIT
        exit 125
    fi
}

flashgate_prepare_work_root() {
    local cache_root
    local resolved_base
    local resolved_home
    local created_root

    flashgate_validate_run_id "${FG_RUN_ID:-}" || {
        printf 'FG_RUN_ID does not match the required restricted format\n' >&2
        return 1
    }
    [[ -n "${HOME:-}" ]] || {
        printf 'HOME is required\n' >&2
        return 1
    }
    for command_name in python3 realpath; do
        command -v "$command_name" >/dev/null 2>&1 || {
            printf 'required command not found: %s\n' "$command_name" >&2
            return 1
        }
    done
    [[ -f "$NATIVE_SAFETY_HELPER" && ! -L "$NATIVE_SAFETY_HELPER" ]] || {
        printf 'safe work-root helper is missing or linked\n' >&2
        return 1
    }

    umask 077
    flashgate_require_real_directory "$HOME" || return 1
    resolved_home="$(realpath -e -- "$HOME")" || return 1
    [[ "$resolved_home" == "$HOME" ]] || {
        printf 'HOME must be canonical and must not contain symbolic links\n' >&2
        return 1
    }

    cache_root="$HOME/.cache"
    if [[ ! -e "$cache_root" ]]; then
        mkdir -- "$cache_root"
    fi
    flashgate_require_real_directory "$cache_root" || return 1

    NATIVE_VALIDATION_BASE="$cache_root/flashgate-mcp-validation"
    if [[ ! -e "$NATIVE_VALIDATION_BASE" ]]; then
        mkdir -- "$NATIVE_VALIDATION_BASE"
    fi
    flashgate_require_real_directory "$NATIVE_VALIDATION_BASE" || return 1
    chmod 700 -- "$NATIVE_VALIDATION_BASE"
    resolved_base="$(realpath -e -- "$NATIVE_VALIDATION_BASE")" || return 1
    [[ "$resolved_base" == "$NATIVE_VALIDATION_BASE" ]] || {
        printf 'validation base must be canonical and link-free\n' >&2
        return 1
    }
    NATIVE_VALIDATION_BASE="$resolved_base"
    NATIVE_WORK_ROOT="$NATIVE_VALIDATION_BASE/$FG_RUN_ID"

    safe_remove_work_root
    created_root="$(
        python3 "$NATIVE_SAFETY_HELPER" create \
            --base "$NATIVE_VALIDATION_BASE" \
            --run-id "$FG_RUN_ID"
    )" || return 1
    [[ "$created_root" == "$NATIVE_WORK_ROOT" ]] || {
        printf 'created work root does not match the expected direct child\n' >&2
        return 1
    }
    [[ "$(realpath -e -- "$NATIVE_WORK_ROOT")" == "$NATIVE_WORK_ROOT" ]] || {
        printf 'work root did not resolve to the expected direct child\n' >&2
        return 1
    }
}
