#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../backend"
# In the default container mode, CI=1 fails if Docker is absent instead of skipping.
CI=1 CHECKIN_FRONTEND_CONTRACT=1 go test -tags=integration -count=1 -v \
  -timeout=5m ./internal/repository -run '^TestCheckinHTTPDatabaseContract$'
