#!/usr/bin/env bash
set -uo pipefail

root_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# shellcheck source=build-input-validation.sh
source "$root_path/scripts/build-input-validation.sh"

binary_path=""
expected_product_version=""
expected_file_version=""
expected_public_arch=""
expected_goarch=""
expected_commit=""
expected_source_time=""
expected_modified=""
skip_execution=false
require_vcs=true
require_go_note=true

errors=()
warnings=()

usage() {
    cat <<'EOF'
Usage:
  scripts/Test-LinuxMetadata.sh [options]

Required:
  --binary PATH
  --expected-product-version VERSION
  --expected-file-version VERSION
  --expected-public-arch x64|arm64
  --expected-goarch amd64|arm64
  --expected-commit SHA40
  --expected-source-time RFC3339
  --expected-modified true|false

Optional:
  --skip-execution
  --allow-missing-vcs
  --allow-missing-go-note
  --help
EOF
}

add_error() {
    errors+=("$1")
}

while (($# > 0)); do
    case "$1" in
        --binary)
            (($# >= 2)) || { add_error "--binary requires a value"; break; }
            binary_path="$2"
            shift 2
            ;;
        --expected-product-version)
            (($# >= 2)) || { add_error "--expected-product-version requires a value"; break; }
            expected_product_version="$2"
            shift 2
            ;;
        --expected-file-version)
            (($# >= 2)) || { add_error "--expected-file-version requires a value"; break; }
            expected_file_version="$2"
            shift 2
            ;;
        --expected-public-arch)
            (($# >= 2)) || { add_error "--expected-public-arch requires a value"; break; }
            expected_public_arch="$2"
            shift 2
            ;;
        --expected-goarch)
            (($# >= 2)) || { add_error "--expected-goarch requires a value"; break; }
            expected_goarch="$2"
            shift 2
            ;;
        --expected-commit)
            (($# >= 2)) || { add_error "--expected-commit requires a value"; break; }
            expected_commit="$2"
            shift 2
            ;;
        --expected-source-time)
            (($# >= 2)) || { add_error "--expected-source-time requires a value"; break; }
            expected_source_time="$2"
            shift 2
            ;;
        --expected-modified)
            (($# >= 2)) || { add_error "--expected-modified requires a value"; break; }
            expected_modified="$2"
            shift 2
            ;;
        --skip-execution)
            skip_execution=true
            shift
            ;;
        --allow-missing-vcs)
            require_vcs=false
            shift
            ;;
        --allow-missing-go-note)
            require_go_note=false
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            add_error "unknown argument: $1"
            shift
            ;;
    esac
done

for required_value in \
    "$binary_path" \
    "$expected_product_version" \
    "$expected_file_version" \
    "$expected_public_arch" \
    "$expected_goarch" \
    "$expected_commit" \
    "$expected_source_time" \
    "$expected_modified"; do
    [[ -n "$required_value" ]] || add_error "one or more required values are missing"
done

for command_name in file readelf go; do
    command -v "$command_name" >/dev/null 2>&1 ||
        add_error "required command not found: $command_name"
done

[[ -f "$binary_path" ]] || add_error "binary not found: $binary_path"
[[ "$expected_goarch" == "amd64" || "$expected_goarch" == "arm64" ]] ||
    add_error "unsupported expected GOARCH: $expected_goarch"
[[ "$expected_public_arch" == "x64" || "$expected_public_arch" == "arm64" ]] ||
    add_error "unsupported expected public architecture: $expected_public_arch"
[[ "$expected_modified" == "true" || "$expected_modified" == "false" ]] ||
    add_error "expected modified value must be true or false"
if ! flashgate_validate_semver "$expected_product_version"; then
    add_error "expected product version is invalid"
elif [[ "$FLASHGATE_FILE_VERSION" != "$expected_file_version" ]]; then
    add_error "expected product/file version mapping is inconsistent"
fi

file_output=""
elf_header=""
elf_notes=""
go_build_info=""
go_build_id=""
static_manifest_output=""
compact_output=""
verbose_output=""

if ((${#errors[@]} == 0)); then
    if ! file_output="$(file -b "$binary_path" 2>&1)"; then
        add_error "file inspection failed: $file_output"
    fi
    if ! elf_header="$(readelf -h "$binary_path" 2>&1)"; then
        add_error "ELF header inspection failed: $elf_header"
    fi
    if ! elf_notes="$(readelf -n "$binary_path" 2>&1)"; then
        add_error "ELF note inspection failed: $elf_notes"
    fi
    if ! go_build_info="$(go version -m "$binary_path" 2>&1)"; then
        add_error "go version -m failed: $go_build_info"
    fi
    if ! go_build_id="$(go tool buildid "$binary_path" 2>&1)"; then
        add_error "go tool buildid failed: $go_build_id"
    fi
    if ! static_manifest_output="$(
        go -C "$root_path" run -mod=vendor ./cmd/versionmanifest \
            --binary "$binary_path" \
            --expected-version "$expected_product_version" \
            --expected-file-version "$expected_file_version" \
            --expected-commit "$expected_commit" \
            --expected-source-time "$expected_source_time" \
            --expected-modified "$expected_modified" \
            --expected-goos linux \
            --expected-goarch "$expected_goarch" \
            --expected-public-arch "$expected_public_arch" 2>&1
    )"; then
        add_error "static build-manifest validation failed: $static_manifest_output"
    fi
fi

if [[ -n "$file_output" ]]; then
    [[ "$file_output" == *"ELF 64-bit"* ]] ||
        add_error "binary is not a 64-bit ELF file: $file_output"
    [[ "$file_output" == *"statically linked"* ]] ||
        add_error "binary is not statically linked: $file_output"
fi

case "$expected_goarch" in
    amd64)
        [[ "$elf_header" == *"Advanced Micro Devices X86-64"* ]] ||
            add_error "ELF machine is not x86-64"
        ;;
    arm64)
        [[ "$elf_header" == *"AArch64"* ]] ||
            add_error "ELF machine is not AArch64"
        ;;
esac

if [[ "$require_go_note" == true ]]; then
    [[ "$elf_notes" == *".note.go.buildid"* ]] ||
        add_error "ELF .note.go.buildid section is missing"
    [[ "$elf_notes" == *"GO BUILDID"* ]] ||
        add_error "ELF Go build-ID note is missing"
    [[ -n "$go_build_id" ]] ||
        add_error "Go build ID is empty"
fi

for expected_build_setting in \
    "path"$'\t'"github.com/thomasweidner/flashgate-mcp/cmd/server" \
    "build"$'\t'"GOOS=linux" \
    "build"$'\t'"GOARCH=${expected_goarch}" \
    "build"$'\t'"CGO_ENABLED=0" \
    "build"$'\t'"-trimpath=true"; do
    [[ "$go_build_info" == *"$expected_build_setting"* ]] ||
        add_error "go version -m is missing: $expected_build_setting"
done

if [[ "$require_vcs" == true ]]; then
    for expected_vcs_setting in \
        "build"$'\t'"vcs=git" \
        "build"$'\t'"vcs.revision=${expected_commit}" \
        "build"$'\t'"vcs.time=${expected_source_time}" \
        "build"$'\t'"vcs.modified=${expected_modified}"; do
        [[ "$go_build_info" == *"$expected_vcs_setting"* ]] ||
            add_error "go version -m is missing VCS setting: $expected_vcs_setting"
    done
fi

if [[ "$skip_execution" == false && ${#errors[@]} -eq 0 ]]; then
    if ! compact_output="$("$binary_path" --version 2>&1)"; then
        add_error "compact version execution failed: $compact_output"
    fi
    if ! verbose_output="$("$binary_path" --version --verbose 2>&1)"; then
        add_error "verbose version execution failed: $verbose_output"
    fi

    [[ "$compact_output" == "flashgate-mcp $expected_product_version" ]] ||
        add_error "unexpected compact version output: $compact_output"

    for expected_line in \
        "Product:      FlashGate MCP" \
        "Version:      $expected_product_version" \
        "File version: $expected_file_version" \
        "Commit:       $expected_commit" \
        "Source time:  $expected_source_time" \
        "Modified:     $expected_modified" \
        "Platform:     linux/$expected_public_arch" \
        "Go target:    linux/$expected_goarch"; do
        [[ "$verbose_output" == *"$expected_line"* ]] ||
            add_error "verbose output is missing: $expected_line"
    done
fi

if ((${#errors[@]} == 0)); then
    status="PASS"
else
    status="FAIL"
fi

printf 'Status: %s\n' "$status"
printf 'BinaryPath: %s\n' "$binary_path"
printf 'ExpectedProductVersion: %s\n' "$expected_product_version"
printf 'ExpectedFileVersion: %s\n' "$expected_file_version"
printf 'ExpectedPublicArch: %s\n' "$expected_public_arch"
printf 'ExpectedGOARCH: %s\n' "$expected_goarch"
printf 'ExpectedCommit: %s\n' "$expected_commit"
printf 'ExpectedSourceTime: %s\n' "$expected_source_time"
printf 'ExpectedModified: %s\n' "$expected_modified"
printf 'FileDescription: %s\n' "$file_output"
printf 'GoBuildID: %s\n' "$go_build_id"
printf 'WarningCount: %d\n' "${#warnings[@]}"
printf 'ErrorCount: %d\n' "${#errors[@]}"
printf 'Warnings: %s\n' "$(IFS='; '; echo "${warnings[*]:-}")"
printf 'Errors: %s\n' "$(IFS='; '; echo "${errors[*]:-}")"

if [[ "$status" != "PASS" ]]; then
    exit 1
fi
