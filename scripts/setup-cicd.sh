#!/usr/bin/env bash
# Run once from a trusted Mac terminal. Passwords are entered only at SSH prompts.
set -euo pipefail
umask 077
host=${1:-175.178.194.116}
user=${2:-ubuntu}
port=${3:-22}
repo=leech-io/cost
[[ "$host" =~ ^[A-Za-z0-9][A-Za-z0-9.-]*$ ]] || exit 1
[[ "$user" =~ ^[a-z_][a-z0-9_-]*$ ]] || exit 1
[[ "$port" =~ ^[0-9]{1,5}$ ]] && ((10#$port > 0 && 10#$port <= 65535)) || exit 1
for tool in gh ssh ssh-keygen python3; do command -v "$tool" >/dev/null; done
gh auth status --hostname github.com
# A private-repository workflow push requires workflow permission for classic OAuth tokens.
gh api "repos/$repo" --jq '.permissions.admin' | python3 -c \
  'import sys; sys.exit(0 if sys.stdin.read().strip() == "true" else "Repository admin permission required")'
scratch=$(mktemp -d "${TMPDIR:-/tmp}/cost-cicd.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
known_name="$host"
[[ "$port" == 22 ]] || known_name="[$host]:$port"
# Reuse the Mac's previously verified key; do not trust an unauthenticated ssh-keyscan.
ssh-keygen -F "$known_name" -f "$HOME/.ssh/known_hosts" > "$scratch/known_hosts"
test -s "$scratch/known_hosts"
options=(-p "$port" -o StrictHostKeyChecking=yes -o "UserKnownHostsFile=$scratch/known_hosts"
  -o ConnectTimeout=10)
remote="$user@$host"
ssh "${options[@]}" "$remote" '
  set -eu
  test -f /home/ubuntu/cost/deploy/.env
  test -w /home/ubuntu/cost
  command -v python3 >/dev/null
  command -v flock >/dev/null
  if docker info >/dev/null 2>&1; then docker compose version; else sudo -n docker info >/dev/null; sudo -n docker compose version; fi
'
ssh-keygen -q -t ed25519 -N '' -C "cost-actions-$(date -u +%Y%m%dT%H%M%SZ)" -f "$scratch/key"
public_key=$(cat "$scratch/key.pub")
printf 'no-agent-forwarding,no-port-forwarding,no-X11-forwarding,no-pty %s\n' "$public_key" > "$scratch/authorized_key"
ssh "${options[@]}" "$remote" \
  'umask 077; mkdir -p ~/.ssh; chmod 700 ~/.ssh; cat >> ~/.ssh/authorized_keys; chmod 600 ~/.ssh/authorized_keys' \
  < "$scratch/authorized_key"
# Verify key-only unattended access before storing any GitHub secrets.
ssh "${options[@]}" -i "$scratch/key" -o IdentitiesOnly=yes -o BatchMode=yes "$remote" 'true'
gh secret set COST_DEPLOY_HOST --repo "$repo" --body "$host"
gh secret set COST_DEPLOY_USER --repo "$repo" --body "$user"
gh secret set COST_DEPLOY_PORT --repo "$repo" --body "$port"
gh secret set COST_DEPLOY_KNOWN_HOSTS --repo "$repo" < "$scratch/known_hosts"
gh secret set COST_DEPLOY_SSH_KEY --repo "$repo" < "$scratch/key"
echo 'CI/CD secrets configured. Merge the workflow into main, then inspect the Actions run.'
echo "Deployment key installed with comment: ${public_key##* }"
