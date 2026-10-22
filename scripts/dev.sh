#!/bin/sh
set -eu
cd "$(dirname "$0")/../backend"
exec ./scripts/dev.sh "$@"
