#!/usr/bin/env bash
worker=verify
source "$(dirname "$0")/common.sh"
# Compose additionally uses network_mode: none and does not mount production
# data or pass credentials. Never accept a destination server or database.
[[ "${COST_ISOLATED_RESTORE:-}" == yes ]] || { echo 'Use the isolated backup-verify Compose service' >&2; exit 1; }
work_dir=${COST_RESTORE_WORK_DIR:-/verify}
[[ "$work_dir" == /* && "$work_dir" != / && "$work_dir" != /var/lib/mysql ]] || exit 1
mkdir -p "$work_dir"
chmod 700 "$work_dir"
scratch=$(mktemp -d "$work_dir/cost-restore.XXXXXX")
server_pid=''
cleanup() {
  local code=$?
  trap - EXIT
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$scratch"
  [[ -z "${report_tmp:-}" ]] || rm -f "$report_tmp"
  if (( code != 0 )); then
    status failed || true
    echo 'Isolated backup restore failed; source backups retained. Check backup health and verifier disk space.' >&2
  fi
  exit "$code"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
latest=''
for file in "$backup_dir"/cost-*.sql.gz; do
  [[ -f "$file" ]] || continue
  if [[ -z "$latest" || "$file" -nt "$latest" ]]; then latest=$file; fi
done
test -n "$latest"
# Copy the immutable published snapshot before retention can remove it.
cp "$latest" "$scratch/source.sql.gz"
gzip -t "$scratch/source.sql.gz" 2>/dev/null
mkdir "$scratch/data"
server_user=$(id -un)
if [[ $(id -u) == 0 ]]; then
  chown mysql:mysql "$work_dir"
  chown -R mysql:mysql "$scratch"
  server_user=mysql
fi
mysqld --no-defaults --initialize-insecure --datadir="$scratch/data" "--user=$server_user" > "$scratch/server.log" 2>&1
mysqld --no-defaults --datadir="$scratch/data" --socket="$scratch/mysql.sock" \
  --pid-file="$scratch/mysql.pid" --skip-networking --mysqlx=0 \
  "--user=$server_user" >> "$scratch/server.log" 2>&1 &
server_pid=$!
client=(--no-defaults --protocol=socket "--socket=$scratch/mysql.sock" -uroot)
ready=0
for ((attempt=0; attempt<60; attempt++)); do
  if mysql "${client[@]}" -e 'SELECT 1' >/dev/null 2>&1; then ready=1; break; fi
  kill -0 "$server_pid" 2>/dev/null || break
  sleep 2
done
test "$ready" == 1
mysql "${client[@]}" -e 'CREATE DATABASE cost CHARACTER SET utf8mb4' > /dev/null 2>&1
gzip -dc "$scratch/source.sql.gz" | mysql "${client[@]}" cost > /dev/null 2> "$scratch/restore.log"
mysqlcheck "${client[@]}" --check cost > "$scratch/check.log" 2>&1
# Query every core table explicitly: an empty/truncated but syntactically valid
# dump must not be reported as restored successfully. Keep totals private.
mkdir -p "$status_dir"
report_tmp="$status_dir/.verify-report.$$"
printf 'backup\t%s\nverified_at_utc\t%s\n' "${latest##*/}" "$(date -u +%FT%TZ)" > "$report_tmp"
mysql "${client[@]}" --batch --skip-column-names cost -e "
SELECT 'schema_versions', GROUP_CONCAT(version ORDER BY version) FROM t_schema_migrations;
SELECT 'categories_count', COUNT(*) FROM t_categories;
SELECT 'budgets_count', COUNT(*) FROM t_budgets;
SELECT 'users_count', COUNT(*) FROM t_users;
SELECT 'sessions_count', COUNT(*) FROM t_sessions;
SELECT 'ledgers_count', COUNT(*) FROM t_ledgers;
SELECT 'family_setup_count', COUNT(*) FROM t_family_setup;
SELECT 'family_invites_count', COUNT(*) FROM t_family_invites;
SELECT 'entries_count', COUNT(*) FROM t_entries;
SELECT 'income_count', COUNT(*) FROM t_entries WHERE kind='income';
SELECT 'income_cent', COALESCE(SUM(amount_cent),0) FROM t_entries WHERE kind='income';
SELECT 'expense_count', COUNT(*) FROM t_entries WHERE kind='expense';
SELECT 'expense_cent', COALESCE(SUM(amount_cent),0) FROM t_entries WHERE kind='expense';
SELECT 'family_members_count', COUNT(*) FROM t_family_members;
SELECT 'accounts_count', COUNT(*) FROM t_accounts;
SELECT 'transfers_count', COUNT(*) FROM t_transfers;
" >> "$report_tmp" 2> "$scratch/report.log"
grep -q '004_ledger_features.sql' "$report_tmp"
mv "$report_tmp" "$status_dir/verify-report.tsv"
status success "${latest##*/}"
echo "Isolated backup restore verified: ${latest##*/}"
