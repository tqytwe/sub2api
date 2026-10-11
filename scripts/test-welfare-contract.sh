#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../backend"
CI=1 WELFARE_FRONTEND_CONTRACT=1 go test -tags=integration -count=1 -v -timeout=10m ./internal/repository -run '^Test(Quiz|Blindbox)(HTTP|Vue|Boundary)DatabaseContract$'
