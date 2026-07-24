#!/usr/bin/env bash
set -Eeuo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fixture_path="$root_path/internal/version/testdata/build-input-validation-fixtures.json"

# shellcheck source=build-input-validation.sh
source "$root_path/scripts/build-input-validation.sh"

errors=()
fixture_stream_complete=false
while IFS= read -r -d '' kind; do
    if ! IFS= read -r -d '' value ||
        ! IFS= read -r -d '' expected; then
        errors+=("fixture stream ended with an incomplete record")
        break
    fi
    printf -v displayed_value '%q' "$value"
    case "$kind" in
        semver-valid)
            if ! flashgate_validate_semver "$value"; then
                errors+=("valid SemVer failed: $displayed_value")
            elif [[ "$FLASHGATE_FILE_VERSION" != "$expected" ]]; then
                errors+=("file version mismatch for $displayed_value")
            fi
            ;;
        semver-invalid)
            if flashgate_validate_semver "$value"; then
                errors+=("invalid SemVer passed: $displayed_value")
            fi
            ;;
        epoch-valid)
            if ! actual="$(flashgate_source_time_from_epoch "$value")"; then
                errors+=("valid epoch failed: $displayed_value")
            elif [[ "$actual" != "$expected" ]]; then
                errors+=("source time mismatch for $displayed_value")
            fi
            ;;
        epoch-invalid)
            if flashgate_validate_source_date_epoch "$value"; then
                errors+=("invalid epoch passed: $displayed_value")
            fi
            ;;
        stream-end)
            fixture_stream_complete=true
            ;;
        *)
            errors+=("unknown fixture record kind: $kind")
            ;;
    esac
done < <(
    python3 - "$fixture_path" <<'PY'
import json
import sys

def emit(kind: str, value: str, expected: str) -> None:
    for field in (kind, value, expected):
        sys.stdout.buffer.write(field.encode("utf-8"))
        sys.stdout.buffer.write(b"\0")

with open(sys.argv[1], encoding="utf-8") as stream:
    data = json.load(stream)
if data.get("schema") != "flashgate-build-input-validation-fixtures/v1":
    raise SystemExit("unexpected fixture schema")
for item in data["semanticVersions"]["valid"]:
    emit("semver-valid", item["value"], item["fileVersion"])
for value in data["semanticVersions"]["invalid"]:
    emit("semver-invalid", value, "")
for item in data["sourceDateEpoch"]["valid"]:
    emit("epoch-valid", item["value"], item["sourceTime"])
for value in data["sourceDateEpoch"]["invalid"]:
    emit("epoch-invalid", value, "")
emit("stream-end", "", "")
PY
)
if [[ "$fixture_stream_complete" != true ]]; then
    errors+=("fixture stream did not complete")
fi

status="PASS"
((${#errors[@]} == 0)) || status="FAIL"
printf 'Status: %s\n' "$status"
printf 'FixturePath: %s\n' "$fixture_path"
printf 'WarningCount: 0\n'
printf 'ErrorCount: %d\n' "${#errors[@]}"
printf 'Warnings:\n'
printf 'Errors: %s\n' "$(IFS='; '; echo "${errors[*]:-}")"
[[ "$status" == "PASS" ]]
