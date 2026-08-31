#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FAIL=0
REGISTRY="$ROOT/docs/FORK_CUSTOMIZATIONS.md"

pass() {
  printf '  [PASS] %s: %s\n' "$1" "$2"
}

fail() {
  printf '  [FAIL] %s: %s\n' "$1" "$2" >&2
  FAIL=1
}

check_file() {
  local id="$1" desc="$2" file="$3"
  if [[ -f "$ROOT/$file" ]]; then pass "$id" "$desc"; else fail "$id" "$desc ($file missing)"; fi
}

check_contains() {
  local id="$1" desc="$2" file="$3" needle="$4"
  if grep -Fq -- "$needle" "$ROOT/$file"; then pass "$id" "$desc"; else fail "$id" "$desc"; fi
}

check_regex() {
  local id="$1" desc="$2" file="$3" pattern="$4"
  if grep -Eq -- "$pattern" "$ROOT/$file"; then pass "$id" "$desc"; else fail "$id" "$desc"; fi
}

check_not_contains() {
  local id="$1" desc="$2" file="$3" needle="$4"
  if ! grep -Fq -- "$needle" "$ROOT/$file"; then pass "$id" "$desc"; else fail "$id" "$desc"; fi
}

check_yaml_event_branch() {
  local id="$1" desc="$2" file="$3" event="$4" branch="$5"
  if node -e '
    const fs = require("fs");
    const lines = fs.readFileSync(process.argv[1], "utf8").split(/\r?\n/);
    const wantedEvent = process.argv[2];
    const wantedBranch = process.argv[3];
    let inOn = false;
    let inEvent = false;
    let inBranches = false;
    let found = false;
    for (const raw of lines) {
      const line = raw.replace(/\s+#.*$/, "");
      if (!line.trim()) continue;
      const indent = line.match(/^ */)[0].length;
      const value = line.trim();
      if (indent === 0) {
        inOn = value === "on:";
        inEvent = false;
        inBranches = false;
        continue;
      }
      if (!inOn) continue;
      if (indent === 2) {
        inEvent = value === `${wantedEvent}:`;
        inBranches = false;
        continue;
      }
      if (!inEvent) continue;
      if (indent === 4) {
        inBranches = value === "branches:";
        continue;
      }
      if (!inBranches || indent !== 6 || !value.startsWith("- ")) continue;
      let branch = value.slice(2).trim();
      const quote = branch.charCodeAt(0);
      if ((quote === 34 || quote === 39) && branch.charCodeAt(branch.length - 1) === quote) {
        branch = branch.slice(1, -1);
      }
      if (branch === wantedBranch) {
        found = true;
        break;
      }
    }
    process.exit(found ? 0 : 1);
  ' "$ROOT/$file" "$event" "$branch"; then
    pass "$id" "$desc"
  else
    fail "$id" "$desc"
  fi
}

run_check() {
  local id="$1" desc="$2"
  shift 2
  if "$@"; then pass "$id" "$desc"; else fail "$id" "$desc"; fi
}

echo "Checking fork registry and static invariants..."

for id in \
  FORK-BRAND-001 FORK-NAV-002 FORK-PLAY-003 FORK-IMAGE-004 FORK-PRICING-005 \
  FORK-DEPLOY-006 FORK-OAUTH-007 FORK-PUBLIC-008 FORK-MIGRATION-009 FORK-BILLING-010 \
  FORK-IMAGE-011 FORK-UI-012 FORK-MARKETPLACE-013 FORK-RISK-013 FORK-ADMIN-014 \
  FORK-REWARDS-015 FORK-MEMBERSHIP-016 FORK-MOBILE-017 FORK-LIVE-SETTLEMENT-018; do
  check_contains "$id" "registry entry exists" "docs/FORK_CUSTOMIZATIONS.md" "## $id"
done

if "$ROOT/scripts/check-jisudeng-branding.sh"; then
  pass "FORK-BRAND-001" "branding protection script"
else
  fail "FORK-BRAND-001" "branding protection script"
fi

check_file "FORK-UI-012" "frontend design system" "docs/FRONTEND_DESIGN_SYSTEM.md"
check_file "FORK-UI-012" "frontend remediation plan" "docs/FRONTEND_EXPERIENCE_REMEDIATION_PLAN.md"
check_file "FORK-UI-012" "machine-readable frontend policy" "docs/frontend-design-governance.json"
check_file "FORK-UI-012" "frontend path-scoped agent rules" "frontend/AGENTS.md"
check_file "FORK-UI-012" "visual review instructions" "docs/visual-reviews/README.md"
check_file "FORK-UI-012" "visual review template" "docs/visual-reviews/TEMPLATE.md"
check_file "FORK-UI-012" "design governance script" "scripts/check-frontend-design-governance.mjs"
check_file "FORK-UI-012" "design governance tests" "scripts/check-frontend-design-governance.test.mjs"
check_contains "FORK-UI-012" "root agent rules require rendered UI review" "AGENTS.md" "任何可见界面改动必须新增一份"
check_contains "FORK-UI-012" "verified frontend build runs design governance" "frontend/package.json" '"build:verified": "pnpm design:verify'
check_contains "FORK-UI-012" "frontend lint runs design governance" "frontend/package.json" '"lint:check": "pnpm design:check'
check_contains "FORK-UI-012" "frontend tests run design governance" "frontend/package.json" '"test:run": "pnpm design:check'
check_contains "FORK-UI-012" "frontend visual changes require prototype images" "docs/FRONTEND_DESIGN_SYSTEM.md" "prototype_artifacts"
check_contains "FORK-UI-012" "visual review template includes prototype artifacts" "docs/visual-reviews/TEMPLATE.md" "prototype_artifacts"
check_contains "FORK-UI-012" "machine policy enforces prototype evidence" "docs/frontend-design-governance.json" '"prototype_visual_evidence": "enforced"'
check_contains "FORK-UI-012" "design governance validates prototype artifacts" "scripts/check-frontend-design-governance.mjs" "prototype_artifacts"
run_check "FORK-UI-012" "design governance self-tests" \
  node --test "$ROOT/scripts/check-frontend-design-governance.test.mjs"
run_check "FORK-UI-012" "design governance command" \
  node "$ROOT/scripts/check-frontend-design-governance.mjs"

check_contains "FORK-NAV-002" "Growth navigation group" "frontend/src/components/layout/AppSidebar.vue" "path: '/growth-group'"
check_not_contains "FORK-NAV-002" "legacy pricing navigation entry removed" "frontend/src/components/layout/AppSidebar.vue" "path: '/pricing'"
check_not_contains "FORK-NAV-002" "user sidebar excludes available channels" "frontend/src/components/layout/AppSidebar.vue" "path: '/available-channels'"
check_contains "FORK-NAV-002" "user sidebar exposes monitor route behind feature flag" "frontend/src/components/layout/AppSidebar.vue" "path: '/monitor', label: t('nav.channelStatus'), icon: SignalIcon, featureFlag: flagChannelMonitor"

check_contains "FORK-PLAY-003" "Play Hub API route" "backend/internal/server/routes/play.go" "authenticated.GET(\"/play/hub\""
check_contains "FORK-PLAY-003" "Play runtime is fail-closed" "backend/internal/service/setting_play_runtime.go" "return PlayRuntime{}"
check_file "FORK-PLAY-003" "Play Hub view" "frontend/src/views/user/PlayHubView.vue"
check_contains "FORK-PLAY-003" "admin team repair candidate route" "backend/internal/server/routes/admin.go" 'play.GET("/teams/:id/member-candidates"'
check_contains "FORK-PLAY-003" "admin team repair write route" "backend/internal/server/routes/admin.go" 'play.POST("/teams/:id/members"'
check_contains "FORK-PLAY-003" "admin team event route" "backend/internal/server/routes/admin.go" 'play.GET("/teams/:id/events"'
check_file "FORK-PLAY-003" "admin team repair service" "backend/internal/service/play_admin_team_repair.go"
check_contains "FORK-PLAY-003" "Chinese admin team repair locale" "frontend/src/i18n/locales/zh/admin/playOps.ts" "memberRepair:"
check_contains "FORK-PLAY-003" "English admin team repair locale" "frontend/src/i18n/locales/en/admin/playOps.ts" "memberRepair:"
check_contains "FORK-PLAY-003" "daily arena reward summary route" "backend/internal/server/routes/play.go" 'play.GET("/arena/daily/reward-summary"'
check_file "FORK-PLAY-003" "daily arena reward summary service" "backend/internal/service/play_daily_reward_summary.go"
check_contains "FORK-PLAY-003" "daily arena summary API client" "frontend/src/api/play.ts" "getArenaDailyRewardSummary"
check_contains "FORK-PLAY-003" "Chinese daily arena summary locale" "frontend/src/i18n/locales/jisudeng-pages.zh.ts" "dailySummary:"
check_contains "FORK-PLAY-003" "English daily arena summary locale" "frontend/src/i18n/locales/jisudeng-pages.en.ts" "dailySummary:"

check_contains "FORK-IMAGE-004" "required prompt error" "backend/internal/service/image_studio.go" "IMAGE_STUDIO_PROMPT_REQUIRED"
check_regex "FORK-IMAGE-004" "prompt hash is private" "backend/internal/service/image_studio.go" 'PromptHash[[:space:]]+string[[:space:]]+`json:"-"`'
check_contains "FORK-IMAGE-004" "authenticated asset download" "backend/internal/server/routes/image_studio.go" 'authenticated.GET("/assets/:id/download"'
check_contains "FORK-IMAGE-004" "mobile support overlay hidden" "frontend/src/router/index.ts" "hideMobileSupport: true"
for asset in ecom-white-bg.webp xhs-cover.webp free-create.webp; do
  check_file "FORK-IMAGE-004" "template asset $asset" "frontend/public/image-studio/templates/$asset"
done

check_file "FORK-IMAGE-011" "Gateway async Redis queue" "backend/internal/repository/image_task_queue.go"
check_contains "FORK-IMAGE-011" "image task terminal CAS" "backend/internal/service/image_task.go" "SaveIfStatus"
check_contains "FORK-IMAGE-011" "image task lease loss" "backend/internal/service/image_task.go" "ErrImageTaskLeaseLost"
check_contains "FORK-IMAGE-011" "private Images result URL" "backend/internal/service/openai_images.go" "IMAGE_RESULT_STORAGE_UNAVAILABLE"
check_contains "FORK-IMAGE-011" "Batch runtime readiness error" "backend/internal/service/batch_image.go" "BATCH_IMAGE_NOT_READY"
check_contains "FORK-IMAGE-011" "admin image runtimes route" "backend/internal/server/routes/admin.go" 'ops.GET("/image-runtimes/health"'
check_contains "FORK-IMAGE-011" "public Images API docs" "frontend/src/content/public-docs-data.zh.ts" "/v1/images/results/{result_id}/{index}"
check_contains "FORK-IMAGE-011" "Zeabur Redis AOF persistence" "deploy/zeabur.template.yaml" "--appendonly yes --appendfsync everysec"
check_contains "FORK-IMAGE-011" "Zeabur persistent data path" "deploy/zeabur.template.yaml" "persistent /data"
check_not_contains "FORK-IMAGE-011" "Zeabur stale app data path removed" "deploy/zeabur.template.yaml" "persistent /app/data"

check_file "FORK-PRICING-005" "model catalog service" "backend/internal/service/model_catalog_service.go"
check_regex "FORK-PRICING-005" "explicit catalog group IDs" "backend/internal/service/model_catalog_types.go" '^[[:space:]]*GroupIDs[[:space:]]+\[\]int64[[:space:]]+`json:"group_ids"`$'
check_contains "FORK-PRICING-005" "site catalog price is display-only" "backend/internal/service/model_pricing_resolver.go" "site catalog as display-only"

check_contains "FORK-DEPLOY-006" "deployment defaults to play/main" "scripts/push-github-and-deploy.sh" 'BRANCH="${1:-play/main}"'
check_contains "FORK-DEPLOY-006" "deployment rejects main" "scripts/push-github-and-deploy.sh" 'if [[ "$BRANCH" == "main" ]]'
check_contains "FORK-DEPLOY-006" "production verification URL" "scripts/push-github-and-deploy.sh" "https://www.jisudeng.com/"
check_file "FORK-DEPLOY-006" "repository delivery rules" "AGENTS.md"
check_file "FORK-DEPLOY-006" "canonical delivery workflow" "docs/DELIVERY_WORKFLOW.md"
check_file "FORK-DEPLOY-006" "delivery verification rule" ".cursor/rules/sub2api-server-only-verify.mdc"
check_contains "FORK-DEPLOY-006" "isolated server worktree is mandatory" "docs/DELIVERY_WORKFLOW.md" "在服务器上从最新审查基线创建独立分支和 Git worktree"
check_contains "FORK-DEPLOY-006" "TDD and per-task reviews are mandatory" "AGENTS.md" "业务行为严格执行 TDD"
check_contains "FORK-DEPLOY-006" "review branch precedes play/main" "docs/DELIVERY_WORKFLOW.md" "审查分支"
check_contains "FORK-DEPLOY-006" "origin play/main remains production source" "docs/DELIVERY_WORKFLOW.md" '生产源码分支是 `origin/play/main`'
check_contains "FORK-DEPLOY-006" "local workstation covers three roles" "docs/DELIVERY_WORKFLOW.md" "用户本地电脑浏览器"
check_contains "FORK-DEPLOY-006" "guest acceptance evidence is recorded" "docs/DELIVERY_WORKFLOW.md" "本地浏览器游客验收："
check_contains "FORK-DEPLOY-006" "regular user acceptance evidence is recorded" "docs/DELIVERY_WORKFLOW.md" "本地浏览器普通用户验收："
check_contains "FORK-DEPLOY-006" "admin acceptance evidence is recorded" "docs/DELIVERY_WORKFLOW.md" "本地浏览器管理员验收："
check_contains "FORK-DEPLOY-006" "credentials stay out of tracked artifacts and logs" "AGENTS.md" "禁止写入 Git、代码、文档、命令输出或日志"
check_not_contains "FORK-DEPLOY-006" "legacy server-only completion wording removed" ".cursor/rules/sub2api-server-only-verify.mdc" "必须通过推送后在 Zeabur 线上验收"
check_yaml_event_branch "FORK-DEPLOY-006" "PRs to play/main trigger fork integrity CI" ".github/workflows/fork-integrity.yml" "pull_request" "play/main"
check_not_contains "FORK-DEPLOY-006" "branch pushes do not duplicate fork integrity CI" ".github/workflows/fork-integrity.yml" "push:"
check_not_contains "FORK-DEPLOY-006" "codex branch pushes do not duplicate fork integrity CI" ".github/workflows/fork-integrity.yml" "codex/**"
check_yaml_event_branch "FORK-DEPLOY-006" "production pushes trigger security scan" ".github/workflows/security-scan.yml" "push" "play/main"
check_yaml_event_branch "FORK-DEPLOY-006" "PRs trigger security scan" ".github/workflows/security-scan.yml" "pull_request" "play/main"
check_not_contains "FORK-DEPLOY-006" "fork CLA automation does not create skipped PR checks" ".github/workflows/cla.yml" "pull_request_target:"
check_not_contains "FORK-DEPLOY-006" "legacy CI does not run on pushes" ".github/workflows/backend-ci.yml" "push:"
check_not_contains "FORK-DEPLOY-006" "legacy CI does not run on PRs" ".github/workflows/backend-ci.yml" "pull_request:"
check_contains "FORK-DEPLOY-006" "full GitHub CI runs once on PR" "docs/DELIVERY_WORKFLOW.md" '完整 GitHub CI 只在目标为 `play/main` 的 PR 上执行一次'
check_contains "FORK-DEPLOY-006" "active release playbook requires local workstation acceptance" "docs/UPSTREAM_SYNC_PLAYBOOK.md" "由用户在本地电脑浏览器"
check_contains "FORK-DEPLOY-006" "legacy rollback plan is overridden by local workstation acceptance" "docs/superpowers/plans/2026-07-15-growth-regression-rollback.md" "2026-07-16 delivery override"
check_contains "FORK-DEPLOY-006" "make test runs the full frontend suite" "Makefile" "pnpm --dir frontend run test:run"

check_contains "FORK-OAUTH-007" "shared OAuth cookie domain" "backend/internal/handler/auth_linuxdo_oauth.go" 'return ".jisudeng.com"'
check_contains "FORK-OAUTH-007" "OAuth domain behavior test" "backend/internal/handler/auth_linuxdo_oauth_test.go" "TestOAuthCookieDomain"

check_file "FORK-RISK-013" "IP risk scoring and privacy model" "backend/internal/service/ip_risk.go"
check_file "FORK-RISK-013" "IP risk runtime and automation gate" "backend/internal/service/ip_risk_service.go"
check_file "FORK-RISK-013" "IP risk raw SQL repository" "backend/internal/repository/ip_risk_repo.go"
check_file "FORK-RISK-013" "IP risk admin repository" "backend/internal/repository/ip_risk_repo_admin.go"
check_file "FORK-RISK-013" "IP risk preview, action and rollback service" "backend/internal/service/ip_risk_admin.go"
check_file "FORK-RISK-013" "IP risk management handler" "backend/internal/handler/admin/ip_risk_handler.go"
check_file "FORK-RISK-013" "IP risk management migration" "backend/migrations/215_ip_risk_management.sql"
check_file "FORK-RISK-013" "IP risk workbench" "frontend/src/features/ip-risk/IPRiskWorkbench.vue"
check_contains "FORK-RISK-013" "risk overview route" "backend/internal/server/routes/admin.go" 'ipRisk.GET("/overview"'
check_contains "FORK-RISK-013" "risk action preview route" "backend/internal/server/routes/admin.go" 'ipRisk.POST("/cases/:id/actions/preview"'
check_contains "FORK-RISK-013" "risk action execution route" "backend/internal/server/routes/admin.go" 'ipRisk.POST("/cases/:id/actions"'
check_contains "FORK-RISK-013" "risk rollback route" "backend/internal/server/routes/admin.go" 'ipRisk.POST("/actions/:id/rollback"'
check_contains "FORK-RISK-013" "migration defaults automatic blocking off" "backend/migrations/215_ip_risk_management.sql" '"auto_block_enabled": false'
check_contains "FORK-RISK-013" "automatic block is temporary" "backend/internal/service/ip_risk_service.go" "RiskActionTemporaryRegistrationBan"
check_contains "FORK-RISK-013" "automatic block targets exact IP" "backend/internal/service/ip_risk_service.go" "ExactIP:   snapshot.Evidence.PrimaryIP"
check_not_contains "FORK-RISK-013" "registration policy checks are not disabled with automation" "backend/internal/service/ip_risk_service.go" "if !autoBlockEnabled"
check_contains "FORK-RISK-013" "preview tokens expire after five minutes" "backend/internal/service/ip_risk_admin.go" "Add(5 * time.Minute)"
check_contains "FORK-RISK-013" "risk actions protect administrators" "backend/internal/service/ip_risk_admin.go" "administrator account is protected"
check_contains "FORK-RISK-013" "risk actions cap users at 500" "backend/internal/service/ip_risk_admin.go" "more than 500 users"
check_contains "FORK-RISK-013" "risk workbench default remains resource tab" "frontend/src/views/admin/ProxiesView.vue" "return 'resources'"
check_contains "FORK-RISK-013" "risk route exists" "frontend/src/router/index.ts" "path: '/admin/proxies/risk'"
check_contains "FORK-RISK-013" "risk action history route exists" "frontend/src/router/index.ts" "path: '/admin/proxies/actions'"
check_contains "FORK-RISK-013" "exact and inferred evidence are separated" "backend/migrations/214_ip_risk_foundation.sql" "evidence_confidence"
check_contains "FORK-RISK-013" "historical OAuth registration is excluded" "backend/internal/repository/ip_risk_repo.go" "path IN ('/api/v1/auth/register', '/api/v1/auth/mobile/register')"

check_contains "FORK-PUBLIC-008" "model plaza route" "backend/internal/server/routes/model_plaza.go" 'plaza.GET(""'
check_not_contains "FORK-PUBLIC-008" "legacy public model pricing route removed" "backend/internal/server/routes/play.go" 'v1.GET("/public/model-pricing"'
check_file "FORK-PUBLIC-008" "public docs content" "frontend/src/content/public-docs-data.zh.ts"
check_contains "FORK-PUBLIC-008" "public model setting" "backend/internal/service/domain_constants.go" "SettingKeyPublicModelsEnabled"

check_file "FORK-MARKETPLACE-013" "marketplace fail-closed runtime" "backend/internal/service/marketplace_runtime.go"
check_file "FORK-MARKETPLACE-013" "marketplace stable errors" "backend/internal/service/marketplace_errors.go"
check_regex "FORK-MARKETPLACE-013" "marketplace setting key" "backend/internal/service/domain_constants.go" '^[[:space:]]*SettingKeyMarketplaceEnabled[[:space:]]*=[[:space:]]*"marketplace_enabled"'
check_regex "FORK-MARKETPLACE-013" "marketplace default is disabled" "backend/internal/service/setting_parse.go" 'SettingKeyMarketplaceEnabled:[[:space:]]*"false"'
check_contains "FORK-MARKETPLACE-013" "public settings bulk reads marketplace key" "backend/internal/service/setting_public.go" "SettingKeyMarketplaceEnabled,"
check_contains "FORK-MARKETPLACE-013" "public DTO exposes marketplace flag" "backend/internal/handler/dto/settings.go" 'MarketplaceEnabled'
check_contains "FORK-MARKETPLACE-013" "public handler maps marketplace value" "backend/internal/handler/setting_handler.go" 'MarketplaceEnabled:'
check_contains "FORK-MARKETPLACE-013" "frontend public settings type" "frontend/src/types/index.ts" 'marketplace_enabled: boolean'
check_contains "FORK-MARKETPLACE-013" "frontend opt-in marketplace flag" "frontend/src/utils/featureFlags.ts" "marketplace: defineFlag"
check_contains "FORK-MARKETPLACE-013" "Chinese marketplace locale" "frontend/src/i18n/locales/zh.ts" "marketplace:"
check_contains "FORK-MARKETPLACE-013" "English marketplace locale" "frontend/src/i18n/locales/en.ts" "marketplace:"
check_not_contains "FORK-MARKETPLACE-013" "generic settings update cannot write marketplace flag" "backend/internal/service/setting_update.go" "SettingKeyMarketplaceEnabled"
check_not_contains "FORK-MARKETPLACE-013" "admin settings DTO excludes marketplace flag" "frontend/src/api/admin/settings.ts" "marketplace_enabled"
check_file "FORK-MARKETPLACE-013" "marketplace scope contract tests" "backend/internal/service/marketplace_scope_contract_test.go"
check_contains "FORK-MARKETPLACE-013" "global role allowlist is exact" "backend/internal/service/admin_user.go" 'supportedGlobalUserRoles = [2]string{RoleUser, RoleAdmin}'
check_not_contains "FORK-MARKETPLACE-013" "user schema does not default to merchant" "backend/ent/schema/user.go" 'Default("merchant")'

MIGRATIONS=(
  170_play_foundation.sql
  171_play_extended.sql
  172_play_retention.sql
  173_play_vip.sql
  174_play_team_affiliate.sql
  175_play_campaigns.sql
  176_play_campaign_name_i18n.sql
  177_marketing_fixes.sql
  177_play_quiz_i18n_and_pool.sql
  178_phase1_growth_world.sql
  179_play_sidebar_defaults.sql
  180_site_subtitle_jisudeng.sql
  181_jisudeng_public_model_pricing.sql
  182_image_studio_asset_storage.sql
  183_model_catalog.sql
  184_image_studio_asset_url_nullable.sql
  185_model_catalog_official_prices.sql
  186_model_catalog_billing_lookup.sql
  186_model_sync_jobs_repair.sql
  187_model_catalog_group_scope.sql
  189_restore_growth_rollback_defaults.sql
  192_image_studio_persistent_jobs.sql
  192_image_studio_persistent_jobs_indexes_notx.sql
  193_image_studio_references.sql
  194_image_studio_asset_derivatives.sql
  194_image_studio_asset_derivatives_indexes_notx.sql
  195_image_studio_billing_reconciliation.sql
  196_image_studio_job_references.sql
  197_image_studio_object_deletions.sql
  198_image_studio_upload_slots.sql
  199_prompt_library.sql
  200_prompt_library_seed.sql
  201_prompt_library_public_seed.sql
  202_prompt_library_generic_cover_cleanup.sql
  203_batch_image_owner_idempotency.sql
  205_vip_recharge_bonus_snapshot.sql
  206_vip_recharge_legacy_tiers_backfill.sql
  207_balance_transactions.sql
  207_subscription_plan_product_display.sql
  208_image_studio_asset_lifecycle.sql
  208_image_studio_asset_lifecycle_indexes_notx.sql
  209_play_arena_daily_reward_summary.sql
  210_subscription_plan_storefront.sql
  210_withdrawable_entitlements.sql
  211_withdrawals.sql
  212_withdrawals_integer_amounts.sql
  213_fund_management_batches.sql
  214_ip_risk_foundation.sql
  215_ip_risk_management.sql
  216_mobile_feedback.sql
  217_billing_surcharge_layer.sql
  234_play_membership_accounting_and_campaign_audience.sql
  235_mobile_feedback_context_and_admin_audit.sql
  236_referral_campaign_core.sql
  237_mobile_attribution.sql
  238_play_membership_operations.sql
  239_membership_financial_fk_guard.sql
  240_mobile_attribution_referral_campaign_fk.sql
  241_play_daily_arena_budget.sql
  241_play_growth_competition.sql
  241_referral_campaign_single_admin_review.sql
  243_play_team_competition.sql
  244_play_arena_period_integrity.sql
  245_referral_campaign_closure.sql
  246_new_user_growth_campaign.sql
  248_restore_upstream_daily_subscription_usage.sql
  248_usage_logs_audio_token_breakdown.sql
  249_live_usage_settlement_outbox.sql
  250_model_catalog_tool_capabilities.sql
  251_model_catalog_official_field_sources.sql
  251_vip_membership_qualification_review.sql
  252_bepusdt_payment_contract.sql
  253_mobile_app_releases.sql
  255_payment_order_coupon_release_processed.sql
  256_payment_order_coupon_release_processed_index_notx.sql
	  259_mobile_video_jobs.sql
	  260_model_catalog_media_capabilities.sql
	  262_play_growth_qualification.sql
	  263_public_status_snapshots.sql
	  264_public_status_ttft_window_index_notx.sql
	  265_play_growth_eligibility_orders_index_notx.sql
	  266_play_growth_reward_snapshot_links.sql
	  267_public_status_ops_aggregation_watermark.sql
	 268_play_growth_governance.sql
	 269_play_membership_manual_contributions.sql
)
for migration in "${MIGRATIONS[@]}"; do
  check_file "FORK-MIGRATION-009" "migration $migration" "backend/migrations/$migration"
  check_contains "FORK-MIGRATION-009" "migration registered in canonical list: $migration" "docs/FORK_CUSTOMIZATIONS.md" "$migration"
done

check_contains "FORK-BILLING-010" "API key ownership validation" "backend/internal/repository/usage_billing_repo.go" "validateUsageBillingOwnership"
check_contains "FORK-BILLING-010" "subscription ownership validation" "backend/internal/repository/usage_billing_repo.go" "validateUsageBillingSubscriptionOwnership"
check_contains "FORK-BILLING-010" "sticky sessions are scoped by API key" "backend/internal/service/gateway_service.go" "scopeStickySessionSeed"
check_contains "FORK-BILLING-010" "recharge completion grants Play boost" "backend/internal/service/payment_fulfillment.go" "GrantRechargeBoost"
check_file "FORK-BILLING-010" "withdrawable entitlement recompute command" "backend/cmd/recompute-withdrawable-entitlements/main.go"
check_file "FORK-BILLING-010" "withdrawable entitlement recompute script" "backend/scripts/recompute-withdrawable-entitlements.sh"
check_contains "FORK-BILLING-010" "image release restores consumed entitlements" "backend/internal/repository/usage_billing_repo.go" "restore_ledger_key"
check_file "FORK-BILLING-010" "withdrawal service" "backend/internal/service/withdrawal.go"
check_contains "FORK-BILLING-010" "user withdrawal create route" "backend/internal/server/routes/user.go" 'wallet.POST("/withdrawals"'
check_contains "FORK-BILLING-010" "admin withdrawal payout route" "backend/internal/server/routes/admin.go" 'withdrawals.POST("/:id/mark-paid"'
check_contains "FORK-BILLING-010" "admin withdrawal step-up route" "backend/internal/server/routes/admin.go" 'withdrawals.GET("/:id/payout-sensitive", gin.HandlerFunc(stepUpAuth)'
check_file "FORK-BILLING-010" "admin withdrawals view" "frontend/src/views/admin/AdminWithdrawalsView.vue"
check_contains "FORK-BILLING-010" "Chinese withdrawal management locale" "frontend/src/i18n/locales/zh.ts" "提现管理"
check_contains "FORK-BILLING-010" "English withdrawal management locale" "frontend/src/i18n/locales/en.ts" "Withdrawals"

check_file "FORK-REWARDS-015" "coupon service" "backend/internal/service/coupon_service.go"
check_contains "FORK-REWARDS-015" "coupon wallet route" "backend/internal/server/routes/user.go" 'authenticated.GET("/coupons/me"'
check_contains "FORK-REWARDS-015" "coupon payment quote route" "backend/internal/server/routes/payment.go" 'authenticated.POST("/coupons/quote"'
check_contains "FORK-REWARDS-015" "daily card one-time quota guard" "backend/internal/service/user_subscription.go" "HasOneTimeDailyQuota"

check_file "FORK-MEMBERSHIP-016" "membership reconciliation" "backend/internal/service/membership_reconciliation.go"
check_contains "FORK-MEMBERSHIP-016" "membership overview route" "backend/internal/server/routes/admin.go" 'play.GET("/membership/overview"'
check_contains "FORK-MEMBERSHIP-016" "VIP publish remains step-up guarded" "backend/internal/server/routes/admin.go" 'play.PUT("/membership/vip-config", gin.HandlerFunc(stepUpAuth)'
check_file "FORK-MEMBERSHIP-016" "membership qualification migration" "backend/migrations/251_vip_membership_qualification_review.sql"

check_contains "FORK-MOBILE-017" "mobile login rate-limited route" "backend/internal/server/routes/auth.go" 'auth.POST("/mobile/login"'
check_contains "FORK-MOBILE-017" "NextChat mobile bootstrap route" "backend/internal/server/routes/nextchat.go" 'authenticated.GET("/mobile/bootstrap"'
check_file "FORK-MOBILE-017" "mobile attribution service" "backend/internal/service/mobile_attribution.go"
check_file "FORK-MOBILE-017" "mobile feedback service" "backend/internal/service/mobile_feedback.go"
check_file "FORK-MOBILE-017" "mobile video capability resolver" "backend/internal/service/mobile_video_availability.go"
check_file "FORK-MOBILE-017" "mobile video private job service" "backend/internal/service/mobile_video_job.go"
check_contains "FORK-MOBILE-017" "mobile video bootstrap route" "backend/internal/server/router.go" 'v1.GET("/mobile/video/bootstrap"'
check_contains "FORK-MOBILE-017" "video session is purpose scoped" "backend/internal/service/mobile_video_session.go" 'NextChatSessionPurposeVideo'
check_contains "FORK-IMAGE-004/FORK-PRICING-005" "media capability catalog field" "backend/internal/service/model_catalog_types.go" 'MediaCapabilities'
check_file "FORK-IMAGE-004/FORK-PRICING-005" "media capability contract" "backend/internal/service/model_media_contract.go"
check_file "FORK-IMAGE-004/FORK-PRICING-005" "media capability migration" "backend/migrations/260_model_catalog_media_capabilities.sql"

check_file "FORK-LIVE-SETTLEMENT-018" "live settlement outbox repository" "backend/internal/repository/live_settlement_outbox_repo.go"
check_file "FORK-LIVE-SETTLEMENT-018" "live settlement recovery service" "backend/internal/service/openai_live_settlement_outbox.go"
check_file "FORK-LIVE-SETTLEMENT-018" "live settlement migration" "backend/migrations/249_live_usage_settlement_outbox.sql"

run_check "DOCS" "local links and document index" node "$ROOT/scripts/check-doc-links.mjs"

if grep -R -n -E '192\.168\.|\./(growth-world-prd|growth-play-roadmap|IMAGE_STUDIO_COMPLETION_PLAN|MODEL_PRICING_PLAN)\.md' \
  "$ROOT/DEV_GUIDE.md" "$ROOT/docs" --exclude-dir=archive; then
  fail "DOCS" "active documents contain obsolete references"
else
  pass "DOCS" "active documents exclude obsolete references"
fi

echo
echo "Running protected backend behaviors..."
run_check "FORK-OAUTH-007" "OAuth cookie domain unit test" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/handler -run '^TestOAuthCookieDomain$'"
run_check "FORK-RISK-013" "IP risk privacy, auth capture, workbench actions and rollback tests" \
  bash -c "cd '$ROOT/backend' && go test -tags=unit -count=1 ./internal/service ./internal/repository ./internal/server/middleware ./internal/handler/admin ./internal/server/routes ./migrations -run 'IPRisk|RiskMetadata|RiskEvent|SuccessfulLoginForwardsRiskEvent'"
run_check "FORK-IMAGE-004/FORK-PRICING-005" "Image Studio and pricing unit tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service -run '^(TestValidateImageStudioPrompt|TestDefaultImageStudioCatalogIncludesPreviewMetadata|TestResolveImageStudioSizeSupportsLegacyAspectAliases|TestInferImageStudioAspectTierIsDeterministic|TestModelCatalogService_.*|TestResolve_SiteCatalogPriceDoesNotAffectBilling|TestResolve_UncataloguedModelKeepsLegacyFallback|TestGenerateSessionHash_MetadataOverridesSessionContext|TestGenerateSessionHash_ResponsesInputDoesNotOverrideHigherPrioritySources)$'"
run_check "FORK-BILLING-010" "billing ownership unit tests" \
  bash -c "cd '$ROOT/backend' && go test -tags=unit -count=1 ./internal/repository -run '^TestValidateUsageBilling.*Ownership'"
run_check "FORK-BILLING-010" "withdrawable ledger and recompute tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service ./migrations -run '^(TestWithdrawable|TestBalanceLedgerGrantArenaDailyCreatesPendingWithdrawableEntitlement|TestBalanceLedgerImageHoldConsumesEntitlementsFIFOWithoutWithdrawalFrozen|TestBalanceLedgerReleaseRestoresOriginalConsumedEntitlementBatches)'"
run_check "FORK-REWARDS-015" "coupon payment lifecycle and daily card quota tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service ./internal/repository -run '^(TestUserSubscription.*DailyCard|TestCheckAndResetWindows_DailyCard.*|TestValidateAndCheckLimits_DailyCard.*|TestCreateOrderInTx.*Coupon|TestPaidCouponOrder.*|TestCouponOrderRefund.*|TestCouponReward.*)'"
run_check "FORK-MEMBERSHIP-016" "membership qualification and VIP tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service ./internal/repository -run '^(TestMembership|TestPaymentOrderMembership|TestBuildPaymentRechargeQuote|Test(Get|Parse|Validate).*VIP)'"
run_check "FORK-MOBILE-017" "mobile protocol, attribution and feedback tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/server/routes ./internal/service -run '^(TestNextChatMobile|TestMobileAttribution|Test.*MobileFeedback)'"
run_check "FORK-IMAGE-004/FORK-PRICING-005/FORK-MOBILE-017" "catalog media and mobile video capability tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service ./internal/handler ./internal/server/routes ./migrations -run '^(TestModelCatalog.*|TestCatalogMobileVideo.*|TestMobileVideo.*|TestGetNextChatWorkspaceModels.*(Media|Video)|TestGatewayModels.*)'"
run_check "FORK-LIVE-SETTLEMENT-018" "live settlement outbox and recovery tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/repository ./internal/service -run '^TestLiveSettlement'"
run_check "FORK-PLAY-003" "admin team repair unit and route tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service ./internal/repository ./internal/handler/admin ./internal/server/routes -run '^(TestAdminTeamRepair|TestAdminTeamMemberCandidatePreview|TestAdminPlayTeamRepairRoutesContract|TestTeamRewardSnapshotLock|TestTeamSettlementSnapshotReusesOuterTransaction)'"
run_check "FORK-PLAY-003" "daily arena reward summary tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service ./internal/repository ./internal/handler ./migrations -run '^(TestDailyArena|TestArenaPeriodQueriesExpose|TestArenaDailyRewardSummaryRoute|TestPlayArenaDailyRewardSummaryMigrationContract)'"
run_check "FORK-IMAGE-011" "Images async, URL and Batch runtime unit tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/repository ./internal/service ./internal/handler -run 'TestImageTask|TestImageRuntimesHealthGatewayAsync|TestAsyncImage|TestOpenAIImageResultServiceRewriteAndEnforceAPIKeyOwnership|TestOpenAIGatewayServiceForwardImages_(APIKeyStreamingURLStoresCompletedImage|OAuthStreamingTransformsEvents|StreamURLRequiresStorageBeforeUpstream)|TestBatchImage(RuntimeState|WorkerRuntime|ProviderRegistryFromConfig|PublicService_Submit)'"
run_check "FORK-MARKETPLACE-013" "marketplace runtime, settings and error unit tests" \
  bash -c "cd '$ROOT/backend' && go test -tags=unit -count=1 ./internal/service ./internal/handler -run '^(TestMarketplace|TestSettingService_(GetPublicSettings.*Marketplace|InitializeDefaultSettings_DefaultsMarketplaceDisabled)|TestSettingHandler_GetPublicSettings_.*Marketplace)' && go test -count=1 ./internal/handler/dto -run '^TestPublicSettingsInjectionPayload_(ContainsMarketplaceEnabled|SchemaDoesNotDrift)$'"

echo
echo "Running protected frontend behaviors..."
run_check "FORK-NAV-002/FORK-IMAGE-004" "sidebar and Image Studio tests" \
  pnpm --dir "$ROOT/frontend" exec vitest run \
    src/components/layout/__tests__/AppSidebar.spec.ts \
    src/components/imageStudio/__tests__/ImageStudioGallery.spec.ts \
    src/components/imageStudio/__tests__/ImageStudioSizePicker.spec.ts \
    src/composables/__tests__/useImageStudioCapabilities.spec.ts \
    src/utils/__tests__/imageStudioWorkspace.spec.ts
run_check "FORK-PLAY-003" "admin team repair and bilingual Play Ops tests" \
  pnpm --dir "$ROOT/frontend" exec vitest run \
    src/api/__tests__/admin.play.teamRepair.spec.ts \
    src/views/admin/__tests__/PlayOpsView.spec.ts \
    src/i18n/locales/adminPlayOpsParity.spec.ts \
    src/i18n/__tests__/bilingualProductUi.spec.ts
run_check "FORK-PLAY-003" "daily arena reward summary frontend test" \
  pnpm --dir "$ROOT/frontend" exec vitest run \
    src/views/public/__tests__/ArenaView.competitive.spec.ts
run_check "FORK-BILLING-010" "wallet withdrawable transparency frontend tests" \
  pnpm --dir "$ROOT/frontend" exec vitest run \
    src/api/__tests__/wallet.spec.ts \
    src/views/user/__tests__/WalletView.spec.ts
run_check "FORK-MARKETPLACE-013" "marketplace feature flag and bilingual locale tests" \
  pnpm --dir "$ROOT/frontend" exec vitest run \
    src/utils/__tests__/featureFlags.spec.ts \
    src/i18n/__tests__/marketplaceLocales.spec.ts
run_check "FORK-RISK-013" "IP risk routes, scanning, preview, stale, partial and rollback frontend tests" \
  pnpm --dir "$ROOT/frontend" exec vitest run \
    src/__tests__/ipRiskRouting.spec.ts \
    src/__tests__/ipRiskWorkbench.spec.ts \
    src/__tests__/ipRiskActions.spec.ts

echo
if [[ "$FAIL" -ne 0 ]]; then
  echo "Fork integrity FAILED. Review $REGISTRY before merging upstream." >&2
  exit 1
fi

echo "Fork integrity passed."
