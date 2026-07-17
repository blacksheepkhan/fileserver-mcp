#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=benchmark-window.sh
source "${script_dir}/benchmark-window.sh"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
build_dir="${repo_root}/build"
server_binary="${build_dir}/flashgate-mcp"
benchmark_binary="${build_dir}/flashgate-benchmark"
budget_path="${repo_root}/benchmarks/budgets.json"
quick=false
record_baseline=false
output_path=""
candidate_path=""
performance_contaminated=false

cleanup() {
  if [[ -n "${candidate_path}" && -f "${candidate_path}" ]]; then
    rm -f -- "${candidate_path}"
  fi
}
trap cleanup EXIT

while [[ $# -gt 0 ]]; do
  case "$1" in
    --quick)
      quick=true
      ;;
    --record-baseline)
      record_baseline=true
      ;;
    --output)
      shift
      output_path="${1:?--output requires a path}"
      ;;
    *)
      printf 'Unknown argument: %s\n' "$1" >&2
      exit 2
      ;;
  esac
  shift
done

if [[ "${record_baseline}" == true ]]; then
  measurement_window_assert_record_allowed
elif measurement_window_is_blocked; then
  performance_contaminated=true
fi

working_tree_dirty="$(git status --porcelain --untracked-files=all)"

if [[ -z "${output_path}" ]]; then
  if [[ "${record_baseline}" == true ]]; then
    output_path="${repo_root}/benchmarks/baseline.linux-$(go env GOARCH).json"
  else
    output_path="${build_dir}/benchmark-current.linux-$(go env GOARCH).json"
  fi
elif [[ "${output_path}" != /* ]]; then
  output_path="${repo_root}/${output_path}"
fi

if [[ "${record_baseline}" == true && -n "${working_tree_dirty}" ]]; then
  printf 'Refusing to record a versioned baseline from a dirty working tree.\n' >&2
  exit 1
fi

mkdir -p "${build_dir}"
go build -o "${server_binary}" ./cmd/server
go build -o "${benchmark_binary}" ./cmd/benchmark
commit="$(git rev-parse HEAD)"

run_output_path="${output_path}"
if [[ "${record_baseline}" == true ]]; then
  candidate_path="$(mktemp "${build_dir}/.benchmark-baseline-candidate.XXXXXXXX.json")"
  run_output_path="${candidate_path}"
fi

arguments=(
  -binary "${server_binary}"
  -output "${run_output_path}"
  -commit "${commit}"
  -budgets "${budget_path}"
)
if [[ -n "${working_tree_dirty}" ]]; then
  arguments+=(-working-tree-dirty)
fi
if [[ "${quick}" == true ]]; then
  arguments+=(-quick)
fi

"${benchmark_binary}" "${arguments[@]}"

if [[ "${record_baseline}" == true ]]; then
  if [[ -n "$(git status --porcelain --untracked-files=all)" ]]; then
    printf 'Refusing final baseline recording because the working tree became dirty.\n' >&2
    exit 1
  fi
  measurement_window_publish_candidate "${candidate_path}" "${output_path}"
  candidate_path=""
elif measurement_window_is_blocked; then
  performance_contaminated=true
fi
if [[ "${performance_contaminated}" == true ]]; then
  measurement_window_contaminated_warning >&2
fi
printf 'Benchmark result: %s\n' "${output_path}"
