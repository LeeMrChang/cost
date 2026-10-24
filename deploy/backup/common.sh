#!/usr/bin/env bash
# Shared by the backup and network-isolated restore workers.
set -Eeuo pipefail
umask 077
backup_dir=${COST_BACKUP_DIR:-/backups}
interval=${COST_BACKUP_INTERVAL_SECONDS:-86400}
retention=${COST_BACKUP_RETENTION_DAYS:-14}
verify_interval=${COST_BACKUP_VERIFY_INTERVAL_SECONDS:-604800}
retry=${COST_BACKUP_RETRY_SECONDS:-300}
for value in "$interval" "$retention" "$verify_interval" "$retry"; do
  [[ "$value" =~ ^[1-9][0-9]{0,7}$ ]] || { echo 'Invalid positive backup interval/retention setting' >&2; exit 1; }
done
status_dir=${COST_BACKUP_STATUS_DIR:-$backup_dir}
status_file="$status_dir/$worker.status"
period=$interval
[[ "$worker" != verify ]] || period=$verify_interval
status() {
  mkdir -p "$status_dir"
  printf '%s %s %s\n' "$1" "$(date +%s)" "${2:--}" > "$status_file.tmp.$$"
  mv "$status_file.tmp.$$" "$status_file"
}
health() {
  local state timestamp detail now
  read -r state timestamp detail < "$status_file" || return 1
  [[ "$state" == success && "$timestamp" =~ ^[0-9]+$ ]] || return 1
  now=$(date +%s)
  (( now >= timestamp && now - timestamp <= period + 3600 ))
}
mode=${1:-loop}
if [[ "$mode" == health ]]; then health; exit; fi
if [[ "$mode" == loop ]]; then
  trap 'exit 0' TERM INT
  while true; do
    # A subprocess preserves errexit through every stage of an attempt.
    delay=$retry
    if bash "$0" once; then delay=$period; fi
    sleep "$delay" &
    wait $! || true
  done
fi
[[ "$mode" == once ]] || { echo 'Usage: worker.sh [loop|once|health]' >&2; exit 2; }
