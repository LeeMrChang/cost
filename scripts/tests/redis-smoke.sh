#!/usr/bin/env bash
# Only disposable GitHub runner resources; never an existing server deployment.
set -Eeuo pipefail
[[ ${GITHUB_ACTIONS:-} == true && -d ${RUNNER_TEMP:-/nonexistent} ]] || { echo 'This smoke test is restricted to a disposable GitHub runner.' >&2; exit 1; }
if docker container inspect cost-cache-redis-1 >/dev/null 2>&1; then
  echo 'Redis container name already exists; refusing to touch it.' >&2
  exit 1
fi
network="cost-redis-ci-${GITHUB_RUN_ID:?}-${GITHUB_RUN_ATTEMPT:?}"
if docker network inspect "$network" >/dev/null 2>&1; then
  echo 'Test network already exists; refusing to reuse it.' >&2
  exit 1
fi
scratch=$(mktemp -d "$RUNNER_TEMP/cost-redis-smoke.XXXXXX")
target="$scratch/redis"
docker network create --internal "$network" >/dev/null
cleanup() {
  code=$?
  trap - EXIT
  set +e
  owner=$(docker container inspect cost-cache-redis-1 --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' 2>/dev/null)
  if [[ "$owner" == "$target" ]]; then docker rm -fv cost-cache-redis-1 >/dev/null; fi
  docker network rm "$network" >/dev/null
  rm -rf "$scratch"
  exit "$code"
}
trap cleanup EXIT
COST_REDIS_NETWORK="$network" bash scripts/setup-cost-redis.sh "$target"
# Re-running the same installation must keep its credentials and container usable.
before=$(sha256sum "$target/.env")
bash scripts/setup-cost-redis.sh "$target"
[[ "$(sha256sum "$target/.env")" == "$before" ]]
unauthenticated=$(docker exec cost-cache-redis-1 redis-cli ping)
[[ "$unauthenticated" == *NOAUTH* ]]
limits=$(docker inspect cost-cache-redis-1 --format '{{.HostConfig.ReadonlyRootfs}} {{.HostConfig.Memory}} {{.HostConfig.NanoCpus}} {{.Config.User}}')
[[ "$limits" == 'true 268435456 500000000 redis' ]]
docker exec cost-cache-redis-1 sh -ec '
  export REDISCLI_AUTH="$REDIS_PASSWORD"
  test "$(redis-cli --raw CONFIG GET maxmemory | tail -n 1)" = 134217728
  test "$(redis-cli --raw CONFIG GET maxmemory-policy | tail -n 1)" = allkeys-lru
  test "$(redis-cli --raw CONFIG GET appendonly | tail -n 1)" = no
  test -z "$(redis-cli --raw CONFIG GET save | tail -n 1)"
  redis-cli SET cost:ci:smoke ok EX 10 >/dev/null
  test "$(redis-cli --raw GET cost:ci:smoke)" = ok
  redis-cli DEL cost:ci:smoke >/dev/null
'
# The GitHub Linux host can reach its own bridge network without publishing Redis.
redis_ip=$(docker inspect cost-cache-redis-1 --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
COST_TEST_REDIS_ADDR="$redis_ip:6379" python3 - "$target/.env" <<'PYTHON'
import os
from pathlib import Path
import subprocess
import sys
values = dict(line.split('=', 1) for line in Path(sys.argv[1]).read_text().splitlines())
env = dict(os.environ, COST_TEST_REDIS_PASSWORD=values['COST_REDIS_PASSWORD'])
subprocess.run(['go', 'test', '-race', '-count=1', './internal/cache', './internal/api', '-run', 'TestRedisIntegration'], cwd='backend', env=env, check=True, timeout=120)
PYTHON
echo 'Redis Docker smoke test passed: authenticated cache, resource limits, no published port, repeatable setup.'
