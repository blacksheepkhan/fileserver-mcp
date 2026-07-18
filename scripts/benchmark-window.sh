#!/usr/bin/env bash

measurement_window_set_status() {
  local epoch="${1:-$(date +%s)}"
  if [[ ! "${epoch}" =~ ^[0-9]+$ ]]; then
    printf 'Invalid test clock value.\n' >&2
    return 2
  fi

  measurement_window_epoch="${epoch}"
  measurement_window_local_time="$(TZ=Europe/Vienna date --date="@${epoch}" '+%Y-%m-%d %H:%M:%S %:z')"
  measurement_window_hour="$((10#$(TZ=Europe/Vienna date --date="@${epoch}" '+%H')))"
  measurement_window_offset="$(TZ=Europe/Vienna date --date="@${epoch}" '+%z')"
  if (( measurement_window_hour >= 19 )); then
    measurement_window_next_date="$(TZ=Europe/Vienna date --date="@$((epoch + 43200))" '+%Y-%m-%d')"
  else
    measurement_window_next_date="$(TZ=Europe/Vienna date --date="@${epoch}" '+%Y-%m-%d')"
  fi
}

measurement_window_is_blocked() {
  measurement_window_set_status "${1:-}" || return
  (( measurement_window_hour >= 19 || measurement_window_hour < 4 ))
}

measurement_window_blocked_message() {
  printf 'Performance baseline recording is blocked at %s Europe/Vienna. Scheduled host-load window: 19:00 inclusive to 04:00 exclusive. Next allowed window starts at %s 04:00 Europe/Vienna; preferred measurement window: 04:15-18:45 Europe/Vienna. No baseline was written or replaced.\n' "${measurement_window_local_time}" "${measurement_window_next_date}"
}

measurement_window_assert_record_allowed() {
  measurement_window_set_status "${1:-}" || return
  if (( measurement_window_hour >= 19 || measurement_window_hour < 4 )); then
    measurement_window_blocked_message >&2
    return 1
  fi
}

measurement_window_publish_candidate() {
  local candidate_path="$1"
  local destination_path="$2"
  local epoch="${3:-}"
  measurement_window_assert_record_allowed "${epoch}" || return
  mv -f -- "${candidate_path}" "${destination_path}"
}

measurement_window_contaminated_warning() {
  printf 'Performance values are contaminated by the scheduled host-load window and are not valid baseline evidence.\n'
}
