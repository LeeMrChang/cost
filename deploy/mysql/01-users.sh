#!/bin/bash
# Sourced by the official MySQL image on first initialization only.
set -e
# Do not enable nounset: this file is sourced into the official entrypoint,
# whose helper functions intentionally read optional unset positional arguments.
for password in "$COST_MYSQL_PASSWORD" "$COST_MIGRATION_PASSWORD"; do
  case "$password" in ''|*[!a-fA-F0-9]*) echo 'Database passwords must be hexadecimal' >&2; exit 1;; esac
  if [ "${#password}" -ne 64 ]; then echo 'Database passwords must contain 64 hex characters' >&2; exit 1; fi
done
docker_process_sql <<SQL
CREATE DATABASE IF NOT EXISTS cost CHARACTER SET utf8mb4;
CREATE USER 'cost'@'%' IDENTIFIED BY '${COST_MYSQL_PASSWORD}';
GRANT SELECT, INSERT, UPDATE, DELETE ON cost.* TO 'cost'@'%';
CREATE USER 'cost_migrator'@'%' IDENTIFIED BY '${COST_MIGRATION_PASSWORD}';
GRANT ALL PRIVILEGES ON cost.* TO 'cost_migrator'@'%';
SQL
