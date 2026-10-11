#!/usr/bin/env bash
# Disposable full-migration PostgreSQL acceptance; never reads application config.
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
command -v docker >/dev/null
command -v go >/dev/null
name="sub2api-p1-counters-$(date +%s)-$$"
cleanup() { docker rm -fv "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
# Fail on an occupied port. Never reuse, reset or remove another database.
docker run --detach --name "$name" --label sub2api.test=subscription-counters \
  -p 127.0.0.1:55432:5432 \
  -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_USER=sub2api_test \
  -e POSTGRES_DB=sub2api_test_migration postgres:18.1-alpine3.23 >/dev/null
ready=false
for _ in {1..60}; do
  if docker exec "$name" pg_isready -U sub2api_test -d sub2api_test_migration >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
if [[ "$ready" != true ]]; then echo 'isolated PostgreSQL failed readiness' >&2; exit 1; fi
export SUB2API_TEST_EXTERNAL_MODE=local
export SUB2API_TEST_POSTGRES_DSN='postgres://sub2api_test@127.0.0.1:55432/sub2api_test_migration?sslmode=disable'
export SUB2API_TEST_REDIS_ADDR=''
cd "$root/backend"
go test -tags=integration,billing_acceptance -count=1 -timeout=15m -v \
  ./internal/billingacceptance -run "${1:-^TestSubscriptionCounterHTTP$}" "${@:2}"
