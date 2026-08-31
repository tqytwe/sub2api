#!/usr/bin/env bash
# 极速蹬品牌完整性检查 — 合并 upstream 后必须 PASS
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FAIL=0

check() {
  local desc="$1"
  shift
  if "$@"; then
    echo "  ✓ $desc"
  else
    echo "  ✗ $desc"
    FAIL=1
  fi
}

check_not() {
  local desc="$1"
  shift
  if ! "$@"; then
    echo "  ✓ $desc"
  else
    echo "  ✗ $desc"
    FAIL=1
  fi
}

echo "Checking jisudeng branding..."

check "AuthLayout uses auth-page" \
  grep -q 'class="auth-page"' "$ROOT/frontend/src/components/layout/AuthLayout.vue"

check "AuthLayout imports jisudeng CSS" \
  grep -q "auth-layout-jisudeng.css" "$ROOT/frontend/src/components/layout/AuthLayout.vue"

check "AuthLayout supports asideMode" \
  grep -q 'asideMode' "$ROOT/frontend/src/components/layout/AuthLayout.vue"

check "AppSidebar has growth-group menu" \
  grep -q "'/growth-group'" "$ROOT/frontend/src/components/layout/AppSidebar.vue"

check "AppSidebar has buildGrowthNavChildren" \
  grep -q 'buildGrowthNavChildren' "$ROOT/frontend/src/components/layout/AppSidebar.vue"

check "AppSidebar hides version badge from non-admin users" \
  grep -q 'VersionBadge v-if="isAdmin"' "$ROOT/frontend/src/components/layout/AppSidebar.vue"

check "AppSidebar keeps available channels out and gates channel status" \
  grep -qv "'/available-channels'" "$ROOT/frontend/src/components/layout/AppSidebar.vue" && \
  grep -q "path: '/monitor', label: t('nav.channelStatus'), icon: SignalIcon, featureFlag: flagChannelMonitor" "$ROOT/frontend/src/components/layout/AppSidebar.vue"

check "tailwind primary is ink (not teal)" \
  grep -q "'#0a0a0a'" "$ROOT/frontend/tailwind.config.js" && \
  ! grep -q '#14b8a6' "$ROOT/frontend/tailwind.config.js"

check "auth-layout-jisudeng.css exists" \
  test -f "$ROOT/frontend/src/styles/auth-layout-jisudeng.css"

check "home-view.css exists" \
  test -f "$ROOT/frontend/src/styles/home-view.css"

check_not "HomeView has no LMSpeed homepage embedding" \
  grep -qi 'lmspeed' "$ROOT/frontend/src/views/HomeView.vue"

check_not "Home document shell has no LMSpeed claim badge" \
  grep -qi 'lmspeed' "$ROOT/frontend/index.html"

if [ "$FAIL" -ne 0 ]; then
  echo ""
  echo "FAILED: jisudeng branding checks — see .cursor/rules/jisudeng-branding-protected.mdc"
  exit 1
fi

echo ""
echo "All jisudeng branding checks passed."
