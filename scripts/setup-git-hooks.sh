#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
if [ ! -f .githooks/pre-push ]; then
	echo "missing .githooks/pre-push; run this script from a full checkout" >&2
	exit 1
fi
chmod +x .githooks/pre-push
git config core.hooksPath .githooks
printf 'Git hooks enabled: core.hooksPath=%s\n' "$(git config core.hooksPath)"
printf 'Direct pushes to main are blocked locally; use pull requests.\n'
