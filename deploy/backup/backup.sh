#!/usr/bin/env bash
worker=backup
source "$(dirname "$0")/common.sh"
mkdir -p "$backup_dir"
chmod 700 "$backup_dir"
scratch=$(mktemp -d "$backup_dir/.backup.partial.XXXXXX")
cleanup() {
  local code=$?
  trap - EXIT
  rm -rf "$scratch"
  if (( code != 0 )); then
    status failed || true
    echo 'Database backup failed; prior successful backups retained. Check database access and free disk space.' >&2
  fi
  exit "$code"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
# SELECT is sufficient for the current InnoDB schema: no locks, tablespaces,
# GTID operations or triggers. App credentials stay in the child environment.
# No SQL, client error output, or secrets are written to logs.
MYSQL_PWD=${COST_MYSQL_PASSWORD:?Database password required} mysqldump \
  --no-defaults --protocol=tcp -h "${COST_BACKUP_DB_HOST:-mysql}" -ucost \
  --single-transaction --quick --no-tablespaces --set-gtid-purged=OFF \
  --skip-triggers --hex-blob --default-character-set=utf8mb4 cost \
  > "$scratch/dump.sql" 2> "$scratch/error.log"
test -s "$scratch/dump.sql"
gzip -c "$scratch/dump.sql" > "$scratch/dump.sql.gz"
gzip -t "$scratch/dump.sql.gz"
name="cost-$(date -u +%Y%m%dT%H%M%SZ)-${scratch##*.}.sql.gz"
# Same filesystem rename exposes a completed file only.
mv "$scratch/dump.sql.gz" "$backup_dir/$name"
# Never expire previous backups until the new one is fully published.
find "$backup_dir" -maxdepth 1 -type f -name 'cost-*.sql.gz' -mmin "+$((retention * 1440))" -delete
status success "$name"
echo "Database backup succeeded: $name"
