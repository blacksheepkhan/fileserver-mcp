#!/usr/bin/env bash
set -Eeuo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
test_root="$(mktemp -d /tmp/flashgate-build-repository.XXXXXXXX)"
linked_root="$test_root/linked"
errors=()

cleanup() {
    if [[ -d "$linked_root" ]]; then
        git -C "$root_path" worktree remove --force "$linked_root" >/dev/null 2>&1 || true
    fi
    if [[ "$test_root" == /tmp/flashgate-build-repository.* && -d "$test_root" ]]; then
        rm -rf -- "$test_root"
    fi
}
trap cleanup EXIT

if ! bash "$root_path/scripts/build.sh" \
    --repository-preflight-only >/dev/null 2>&1; then
    errors+=("regular repository preflight failed")
fi

if ! git -C "$root_path" worktree add \
    --detach \
    "$linked_root" \
    HEAD >/dev/null 2>&1; then
    errors+=("unable to create linked-worktree fixture")
else
    cp -- "$root_path/scripts/build.sh" "$linked_root/scripts/build.sh"
    cp -- \
        "$root_path/scripts/build-input-validation.sh" \
        "$linked_root/scripts/build-input-validation.sh"
    if ! bash "$linked_root/scripts/build.sh" \
        --repository-preflight-only >/dev/null 2>&1; then
        errors+=("linked-worktree preflight failed")
    fi
fi

for fixture in nonrepository corrupted; do
    fixture_root="$test_root/$fixture"
    mkdir -p "$fixture_root/scripts"
    cp -- "$root_path/scripts/build.sh" "$fixture_root/scripts/build.sh"
    cp -- \
        "$root_path/scripts/build-input-validation.sh" \
        "$fixture_root/scripts/build-input-validation.sh"
done
printf 'gitdir: missing-git-directory\n' >"$test_root/corrupted/.git"

if bash "$test_root/nonrepository/scripts/build.sh" \
    --repository-preflight-only >/dev/null 2>&1; then
    errors+=("non-repository fixture unexpectedly passed")
fi
if bash "$test_root/corrupted/scripts/build.sh" \
    --repository-preflight-only >/dev/null 2>&1; then
    errors+=("corrupted Git metadata fixture unexpectedly passed")
fi

status="PASS"
((${#errors[@]} == 0)) || status="FAIL"
printf 'Status: %s\n' "$status"
printf 'TestRoot: %s\n' "$test_root"
printf 'WarningCount: 0\n'
printf 'ErrorCount: %d\n' "${#errors[@]}"
printf 'Warnings:\n'
printf 'Errors: %s\n' "$(IFS='; '; echo "${errors[*]:-}")"
[[ "$status" == "PASS" ]]
