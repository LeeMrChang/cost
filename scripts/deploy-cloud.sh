#!/usr/bin/env bash
# Executed over SSH. Requires an existing, initialized production deployment.
set -Eeuo pipefail
umask 077
release_id=${1:?release ID required}
root=${2:-/home/ubuntu/cost}
[[ "$release_id" =~ ^[a-f0-9]{40}-[0-9]+-[0-9]+$ ]] || { echo 'Invalid release ID' >&2; exit 1; }
[[ "$root" =~ ^/[A-Za-z0-9_./-]+$ && "$root" != / ]] || { echo 'Invalid deployment root' >&2; exit 1; }
test -f "$root/deploy/.env"
exec 9>"$root/deploy.lock"
flock -n 9 || { echo 'Another deployment is running' >&2; exit 1; }
release="$root/releases/$release_id"
archive="$root/incoming/$release_id.tar.gz"
if [[ -f "$release/SUCCESS" ]]; then
  echo "Release $release_id already deployed"
  exit 0
fi
mkdir -p "$root/releases" "$root/backups"

# Reject traversal, links, duplicate paths, secrets and files outside the packager's whitelist.
python3 - "$archive" "$release" <<'PY'
import pathlib, shutil, sys, tarfile
archive, target = sys.argv[1:]
fixed = {'backend/Dockerfile', 'backend/.dockerignore', 'backend/go.mod', 'backend/go.sum',
         'deploy/compose.cloud.yaml', 'deploy/.env.example', 'deploy/Caddyfile',
         'deploy/mysql/01-users.sh', 'docs/ops/2026-10-22-docker-cloud-deployment.md'}
with tarfile.open(archive, 'r:gz') as tf:
    members, seen = tf.getmembers(), set()
    for m in members:
        p = pathlib.PurePosixPath(m.name)
        source = (m.name.startswith(('backend/cmd/', 'backend/internal/'))
                  and p.suffix in ('.go', '.sql') and not m.name.endswith('_test.go'))
        if (not m.isfile() or p.is_absolute() or '..' in p.parts or '\\' in m.name
                or m.name in seen or not (m.name in fixed or source)):
            raise SystemExit('Unsafe or unexpected deployment archive member')
        seen.add(m.name)
    if not {'backend/go.mod', 'deploy/compose.cloud.yaml'} <= seen:
        raise SystemExit('Incomplete deployment archive')
    base = pathlib.Path(target)
    base.mkdir(mode=0o700)  # A failed release is never silently overwritten.
    for m in members:
        dest = base / m.name
        dest.parent.mkdir(parents=True, exist_ok=True)
        with tf.extractfile(m) as src, dest.open('xb') as out:
            shutil.copyfileobj(src, out)
PY

DOCKER=(docker)
if ! docker info >/dev/null 2>&1; then
  # sudo normally drops COST_IMAGE_TAG; pass this non-secret value explicitly.
  DOCKER=(sudo -n env "COST_IMAGE_TAG=ci-$release_id" docker)
  "${DOCKER[@]}" info >/dev/null
fi
export COST_IMAGE_TAG="ci-$release_id"
compose() {
  "${DOCKER[@]}" compose --project-name cost-cloud --env-file "$root/deploy/.env" \
    -f "$release/deploy/compose.cloud.yaml" "$@" < /dev/null
}
old_api=''
stopped=0
migration_started=0
verify_container="cost-ci-verify-$release_id"
verify_created=0
cleanup() {
  status=$?
  trap - EXIT
  set +e
  if [[ "$verify_created" == 1 ]]; then
    "${DOCKER[@]}" rm -fv "$verify_container" >/dev/null
  fi
  if [[ "$status" != 0 && "$stopped" == 1 ]]; then
    if [[ "$migration_started" == 0 ]]; then
      "${DOCKER[@]}" start "$old_api" >/dev/null
      echo 'Deployment failed before migration; original API restart requested.' >&2
    else
      compose stop api >/dev/null
      echo 'Deployment failed after migration began. API stopped; inspect backup and migration before recovery.' >&2
    fi
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

compose config --quiet
mysql_id=$(compose ps --status running -q mysql)
old_api=$(compose ps --status running -q api)
test -n "$mysql_id" && test -n "$old_api"
mysql_image=$("${DOCKER[@]}" inspect --format '{{.Image}}' "$mysql_id")
old_image=$("${DOCKER[@]}" inspect --format '{{.Image}}' "$old_api")
printf '%s\n' "$old_image" > "$release/PREVIOUS_IMAGE"
# Preserve the previous image even if a future build reuses a local tag.
"${DOCKER[@]}" tag "$old_image" "cost-api:before-$release_id"
compose build api

stopped=1
compose stop api
backup="$root/backups/ci-$release_id.sql"
compose exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --no-tablespaces --set-gtid-purged=OFF --hex-blob cost' > "$backup"
test -s "$backup"

# Restore on the identical MySQL image with no network and an isolated anonymous volume.
verify_created=1
"${DOCKER[@]}" run -d --name "$verify_container" --network none \
  -e MYSQL_ALLOW_EMPTY_PASSWORD=yes "$mysql_image" >/dev/null
ready=0
for ((attempt=0; attempt<60; attempt++)); do
  if "${DOCKER[@]}" exec "$verify_container" mysql -uroot -e 'SELECT 1' >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
test "$ready" == 1
"${DOCKER[@]}" exec "$verify_container" mysql -uroot -e 'CREATE DATABASE cost CHARACTER SET utf8mb4'
"${DOCKER[@]}" exec -i "$verify_container" mysql -uroot cost < "$backup"
hash_stream() {
  python3 -c 'import hashlib,sys
h = hashlib.sha256()
for chunk in iter(lambda: sys.stdin.buffer.read(1048576), b""):
    h.update(chunk)
print(h.hexdigest())'
}
dump_args='--no-create-info --skip-comments --compact --skip-extended-insert --order-by-primary --skip-add-locks --skip-disable-keys --no-tablespaces --set-gtid-purged=OFF --hex-blob cost'
source_hash=$(compose exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot '"$dump_args" | hash_stream)
restore_hash=$("${DOCKER[@]}" exec "$verify_container" sh -c "exec mysqldump -uroot $dump_args" | hash_stream)
[[ "$source_hash" == "$restore_hash" ]] || { echo 'Restored backup data mismatch' >&2; exit 1; }
"${DOCKER[@]}" rm -fv "$verify_container" >/dev/null
verify_created=0

migration_started=1
# Existing MySQL stays running: do not recreate it or apply unrelated database-image changes.
compose --profile tools run --rm --no-deps -T migrate
compose up -d --no-deps --wait --wait-timeout 120 api
curl --fail --silent --show-error --max-time 10 http://127.0.0.1:8080/health >/dev/null
printf 'COST_IMAGE_TAG=%s\n' "$COST_IMAGE_TAG" > "$release/image.env"
python3 - "$root" "$release" <<'PY'
import os, sys
root, release = sys.argv[1:]
next_link = root + '/current.next'
os.symlink(release, next_link)
os.replace(next_link, root + '/current')
PY
touch "$release/SUCCESS"
echo "Deployed $release_id; backup retained on server."
