#!/usr/bin/env bash
# Push play/main to GitHub; Zeabur (or other) auto-deploys from the remote.
# Usage: ./scripts/push-github-and-deploy.sh [branch]
# Default branch is play/main (NOT origin/main — that is unrelated upstream history).
set -euo pipefail

BRANCH="${1:-play/main}"
REMOTE="${SUB2API_GITHUB_REMOTE:-origin}"
VERIFY_URL="${SUB2API_VERIFY_URL:-https://jisuodeng.zeabur.app/}"

if [[ "$BRANCH" == "main" ]]; then
  echo "error: refusing to push branch 'main'" >&2
  echo "  Local play/main tracks origin/play/main; origin/main is unrelated upstream history." >&2
  echo "  Use: ./scripts/push-github-and-deploy.sh" >&2
  echo "  Or:  ./scripts/push-github-and-deploy.sh play/main" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "error: not a git repository — run git init first" >&2
  exit 1
fi

echo "==> push ${REMOTE}/${BRANCH}"
git push -u "$REMOTE" "$BRANCH"

echo "==> done — wait for remote build, then verify ${VERIFY_URL}"
