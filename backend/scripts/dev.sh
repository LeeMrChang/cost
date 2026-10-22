#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi
if command -v go >/dev/null 2>&1; then
  exec go run ./cmd/api "$@"
fi
if [ -x /tmp/cost-toolchain/go/bin/go ]; then
  export GOPATH=/tmp/cost-go-path
  export GOCACHE=/tmp/cost-go-cache
  export GOPROXY=https://goproxy.cn
  exec /tmp/cost-toolchain/go/bin/go run ./cmd/api "$@"
fi
printf '%s\n' '未找到 Go，请安装 Go 1.23 或更新版本。' >&2
exit 1
