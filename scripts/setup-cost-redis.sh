#!/usr/bin/env bash
# Run on the Docker host. Creates only the separate cost-cache Redis service.
set +x
set -Eeuo pipefail
umask 077
target=${1:-/home/ubuntu/cost/redis}
[[ "$target" == /* && "$target" != / && ! -L "$target" ]] || { echo 'Use an absolute, non-symlink installation directory.' >&2; exit 1; }
target=$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$target")
source_config="$(cd "$(dirname "$0")/.." && pwd)/deploy/redis/compose.yaml"
test -f "$source_config"
image_ref=${COST_REDIS_IMAGE:-redis:8.4.8-alpine}
network_name=${COST_REDIS_NETWORK:-cost-cloud_default}
# Compose must use the saved server-only environment, not inherited overrides.
unset COST_REDIS_PASSWORD COST_REDIS_IMAGE COST_REDIS_NETWORK

# The fixed Compose project needs one host-wide lock, even for different targets.
lock_file=/tmp/cost-cache-install.lock
[[ ! -L "$lock_file" && (! -e "$lock_file" || -f "$lock_file") ]] || { echo 'Unsafe Redis installer lock path.' >&2; exit 1; }
exec 9>>"$lock_file"
flock -n 9 || { echo 'Another Redis installation is running.' >&2; exit 1; }

if [[ -e "$target" && ! -f "$target/.cost-redis-managed" ]]; then
  [[ -d "$target" && -z "$(ls -A "$target")" ]] || { echo 'Refusing to overwrite an unmanaged installation directory.' >&2; exit 1; }
fi
DOCKER=(docker)
if ! docker info >/dev/null 2>&1; then
  DOCKER=(sudo -n docker)
  "${DOCKER[@]}" info >/dev/null 2>&1 || { echo 'Docker access unavailable; no service was changed.' >&2; exit 1; }
fi

if [[ -f "$target/.cost-redis-managed" ]]; then
  cmp -s "$source_config" "$target/compose.yaml" || { echo 'Installed Compose differs; review it before upgrading.' >&2; exit 1; }
  # Validate without sourcing a shell file or printing its password.
  network_name=$(python3 - "$target/.env" <<'PY'
import pathlib, re, sys
try:
    lines = pathlib.Path(sys.argv[1]).read_text().splitlines()
    pairs = [line.split('=', 1) for line in lines]
    values = dict(pairs)
    assert len(values) == len(pairs) == 3
    assert set(values) == {'COST_REDIS_PASSWORD', 'COST_REDIS_IMAGE', 'COST_REDIS_NETWORK'}
    assert re.fullmatch(r'[0-9a-f]{64}', values['COST_REDIS_PASSWORD'])
    assert re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9./:_@-]*', values['COST_REDIS_IMAGE'])
    assert re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]*', values['COST_REDIS_NETWORK'])
except (OSError, ValueError, AssertionError, KeyError):
    sys.exit('Saved Redis environment is invalid; no service was changed.')
print(values['COST_REDIS_NETWORK'])
PY
  )
fi
[[ "$image_ref" =~ ^[A-Za-z0-9][A-Za-z0-9./:_@-]*$ && "$network_name" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || { echo 'Invalid Redis image or network name.' >&2; exit 1; }
"${DOCKER[@]}" network inspect "$network_name" >/dev/null 2>&1 || { echo 'Cost Docker network is missing; check its actual name first.' >&2; exit 1; }
check_owner() {
  if existing=$("${DOCKER[@]}" container inspect cost-cache-redis-1 --format '{{index .Config.Labels "com.docker.compose.project"}}|{{index .Config.Labels "com.docker.compose.service"}}|{{index .Config.Labels "com.docker.compose.project.working_dir"}}|{{index .Config.Labels "com.docker.compose.project.config_files"}}' 2>/dev/null); then
    [[ -f "$target/.cost-redis-managed" && "$existing" == "cost-cache|redis|$target|$target/compose.yaml" ]] || { echo 'Container is owned by another installation; refusing takeover.' >&2; exit 1; }
  fi
}
check_owner

mkdir -p "$target"
chmod 700 "$target"
check_owner
if [[ ! -f "$target/.cost-redis-managed" ]]; then
  # Exclusive environment creation preserves even partially initialized secrets.
  password=$(openssl rand -hex 32)
  (set -o noclobber; printf 'COST_REDIS_PASSWORD=%s\nCOST_REDIS_IMAGE=%s\nCOST_REDIS_NETWORK=%s\n' "$password" "$image_ref" "$network_name" > "$target/.env")
  unset password
  cp "$source_config" "$target/compose.yaml"
  touch "$target/.cost-redis-managed"
fi
chmod 600 "$target/.env"
compose() {
  "${DOCKER[@]}" compose --project-name cost-cache --env-file "$target/.env" -f "$target/compose.yaml" "$@" < /dev/null
}
compose config --quiet
compose up -d --no-deps --wait --wait-timeout 90 redis
reply=$(compose exec -T redis sh -ec 'REDISCLI_AUTH="$REDIS_PASSWORD" redis-cli -h 127.0.0.1 ping')
[[ "$reply" == PONG ]] || { echo 'Authenticated Redis PING failed.' >&2; exit 1; }
published_ports=$("${DOCKER[@]}" port cost-cache-redis-1)
if [[ -n "$published_ports" ]]; then
  compose stop redis
  echo 'Unexpected published port; only this Redis service was stopped.' >&2
  exit 1
fi
echo "Redis ready: cost-cache-redis-1; internal address cost-redis:6379; credentials retained in $target/.env (not printed)."
echo 'Cost API is unchanged. Cache integration and performance measurements remain separate steps.'
