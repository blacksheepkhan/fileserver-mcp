#!/usr/bin/env bash
set -uo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=benchmark-window.sh
source "${script_dir}/benchmark-window.sh"

failures=()
check_count=0
temporary_directory=""

check() {
  local condition="$1"
  local name="$2"
  check_count=$((check_count + 1))
  if [[ "${condition}" != true ]]; then
    failures+=("${name}")
  fi
}

epoch() {
  date -u --date="$1" +%s
}

run_case() {
  local instant="$1"
  local expected="$2"
  local name="$3"
  local actual=false
  if measurement_window_is_blocked "$(epoch "${instant}")"; then
    actual=true
  fi
  check "$([[ "${actual}" == "${expected}" ]] && printf true || printf false)" "${name}"
}

run_case '2026-01-17 02:59:00Z' true '03:59 blocked'
run_case '2026-01-17 03:00:00Z' false '04:00 allowed'
run_case '2026-01-17 03:15:00Z' false '04:15 allowed'
run_case '2026-07-17 16:45:00Z' false '18:45 allowed'
run_case '2026-07-17 16:59:00Z' false '18:59 allowed'
run_case '2026-07-17 17:00:00Z' true '19:00 blocked'
run_case '2026-07-17 21:59:00Z' true '23:59 blocked'

measurement_window_set_status "$(epoch '2026-01-17 12:00:00Z')"
check "$([[ "${measurement_window_offset}" == '+0100' ]] && printf true || printf false)" 'winter UTC offset'
measurement_window_set_status "$(epoch '2026-07-17 12:00:00Z')"
check "$([[ "${measurement_window_offset}" == '+0200' ]] && printf true || printf false)" 'summer UTC offset'

TZ=UTC
export TZ
measurement_window_set_status "$(epoch '2026-07-17 17:00:00Z')"
check "$([[ "${measurement_window_hour}" == 19 ]] && printf true || printf false)" 'system TZ does not alter Europe/Vienna'

blocked_rejected=false
blocked_message="$(measurement_window_assert_record_allowed "$(epoch '2026-07-17 17:00:00Z')" 2>&1)"
if [[ $? -ne 0 ]]; then
  blocked_rejected=true
fi
check "${blocked_rejected}" 'blocked record precheck'
blocked_message_complete=false
if [[ "${blocked_message}" == *'2026-07-17 19:00:00 +02:00'* && "${blocked_message}" == *'19:00 inclusive to 04:00 exclusive'* && "${blocked_message}" == *'2026-07-18 04:00 Europe/Vienna'* && "${blocked_message}" == *'No baseline was written or replaced.'* ]]; then
  blocked_message_complete=true
fi
check "${blocked_message_complete}" 'blocked message contains current time, window, and next window'

allowed_accepted=false
if measurement_window_assert_record_allowed "$(epoch '2026-07-17 02:15:00Z')" >/dev/null 2>&1; then
  allowed_accepted=true
fi
check "${allowed_accepted}" 'allowed record precheck'

temporary_directory="$(mktemp -d)"
candidate="${temporary_directory}/candidate.json"
existing="${temporary_directory}/baseline.json"
printf 'candidate' > "${candidate}"
printf 'existing' > "${existing}"
publish_rejected=false
if ! measurement_window_publish_candidate "${candidate}" "${existing}" "$(epoch '2026-07-17 17:00:00Z')" >/dev/null 2>&1; then
  publish_rejected=true
fi
check "${publish_rejected}" 'publication recheck blocks at 19:00'
check "$([[ -f "${candidate}" ]] && printf true || printf false)" 'blocked candidate retained for caller cleanup'
check "$([[ "$(<"${existing}")" == existing ]] && printf true || printf false)" 'existing baseline not replaced'
check "$([[ "$(measurement_window_contaminated_warning)" == 'Performance values are contaminated by the scheduled host-load window and are not valid baseline evidence.' ]] && printf true || printf false)" 'normal-run contamination warning'

rm -f -- "${candidate}" "${existing}"
rmdir -- "${temporary_directory}"

if (( ${#failures[@]} == 0 )); then
  printf 'Status       : PASS\nCheckCount   : %d\nFailureCount : 0\nFailures     :\n' "${check_count}"
  exit 0
fi
printf 'Status       : FAIL\nCheckCount   : %d\nFailureCount : %d\nFailures     : %s\n' "${check_count}" "${#failures[@]}" "${failures[*]}"
exit 1
