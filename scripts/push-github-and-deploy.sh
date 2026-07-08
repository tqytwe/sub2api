#!/usr/bin/env bash
# Push local main to GitHub, then pull + rebuild on Dell.
# Usage: ./scripts/push-github-and-deploy.sh [branch]
set -euo pipefail

BRANCH="${1:-play/main}"
REMOTE="${SUB2API_GITHUB_REMOTE:-origin}"
SERVER="${SUB2API_DEPLOY_HOST:-dell@192.168.100.10}"
SERVER_DIR="${SUB2API_DEPLOY_DIR:-/data/1panel/sub2api}"
COMPOSE_FILE="docker-compose.server.yml"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "error: not a git repository — run git init first" >&2
  exit 1
fi

echo "==> push ${REMOTE}/${BRANCH}"
git push -u "$REMOTE" "$BRANCH"

echo "==> deploy on ${SERVER}:${SERVER_DIR}"
ssh "$SERVER" bash -s <<EOF
set -euo pipefail
cd "${SERVER_DIR}"
if [ ! -d .git ]; then
  echo "error: ${SERVER_DIR} is not a git clone" >&2
  exit 1
fi
git fetch origin
git checkout "${BRANCH}"
git pull --ff-only origin "${BRANCH}"
cd deploy
if [ ! -f .env ]; then
  echo "error: deploy/.env missing — copy .env.server.example and set secrets" >&2
  exit 1
fi
docker compose -f "${COMPOSE_FILE}" up -d --build
docker compose -f "${COMPOSE_FILE}" ps
set -a
source .env
set +a
curl -sfS -o /dev/null -w "health=%{http_code}\n" "http://127.0.0.1:${SERVER_PORT:-8206}/health" || true
EOF

echo "==> done — open http://192.168.100.10:8206"
