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
  FORK-IMAGE-011; do
  check_contains "$id" "registry entry exists" "docs/FORK_CUSTOMIZATIONS.md" "## $id"
done

if "$ROOT/scripts/check-jisudeng-branding.sh"; then
  pass "FORK-BRAND-001" "branding protection script"
else
  fail "FORK-BRAND-001" "branding protection script"
fi

check_contains "FORK-NAV-002" "Growth navigation group" "frontend/src/components/layout/AppSidebar.vue" "path: '/growth-group'"
check_contains "FORK-NAV-002" "models navigation entry" "frontend/src/components/layout/AppSidebar.vue" "path: '/models'"
check_not_contains "FORK-NAV-002" "user sidebar excludes available channels" "frontend/src/components/layout/AppSidebar.vue" "path: '/available-channels'"
check_not_contains "FORK-NAV-002" "user sidebar excludes monitor route" "frontend/src/components/layout/AppSidebar.vue" "path: '/monitor'"

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
check_contains "FORK-PRICING-005" "explicit catalog group IDs" "backend/internal/service/model_catalog_types.go" 'GroupIDs                []int64    `json:"group_ids"`'
check_contains "FORK-PRICING-005" "site catalog price precedence" "backend/internal/service/model_pricing_resolver.go" "firstCatalogPrice"

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

check_contains "FORK-PUBLIC-008" "public model route" "backend/internal/server/routes/play.go" 'v1.GET("/public/model-pricing"'
check_file "FORK-PUBLIC-008" "public docs content" "frontend/src/content/public-docs-data.zh.ts"
check_contains "FORK-PUBLIC-008" "public model setting" "backend/internal/service/domain_constants.go" "SettingKeyPublicModelsEnabled"

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
)
for migration in "${MIGRATIONS[@]}"; do
  check_file "FORK-MIGRATION-009" "migration $migration" "backend/migrations/$migration"
  check_contains "FORK-MIGRATION-009" "migration registered in canonical list: $migration" "docs/FORK_CUSTOMIZATIONS.md" "$migration"
done

check_contains "FORK-BILLING-010" "API key ownership validation" "backend/internal/repository/usage_billing_repo.go" "validateUsageBillingOwnership"
check_contains "FORK-BILLING-010" "subscription ownership validation" "backend/internal/repository/usage_billing_repo.go" "validateUsageBillingSubscriptionOwnership"
check_contains "FORK-BILLING-010" "sticky sessions are scoped by API key" "backend/internal/service/gateway_service.go" "scopeStickySessionSeed"
check_contains "FORK-BILLING-010" "recharge completion grants Play boost" "backend/internal/service/payment_fulfillment.go" "GrantRechargeBoost"

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
run_check "FORK-IMAGE-004/FORK-PRICING-005" "Image Studio and pricing unit tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service -run '^(TestValidateImageStudioPrompt|TestDefaultImageStudioCatalogIncludesPreviewMetadata|TestResolveImageStudioSizeSupportsLegacyAspectAliases|TestInferImageStudioAspectTierIsDeterministic|TestModelCatalogService_.*|TestResolve_SiteCatalogPriceWinsOverLegacyFallback|TestResolve_UncataloguedModelKeepsLegacyFallback|TestGenerateSessionHash_MetadataOverridesSessionContext|TestGenerateSessionHash_ResponsesInputDoesNotOverrideHigherPrioritySources)$'"
run_check "FORK-BILLING-010" "billing ownership unit tests" \
  bash -c "cd '$ROOT/backend' && go test -tags=unit -count=1 ./internal/repository -run '^TestValidateUsageBilling.*Ownership'"
run_check "FORK-PLAY-003" "admin team repair unit and route tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service ./internal/repository ./internal/handler/admin ./internal/server/routes -run '^(TestAdminTeamRepair|TestAdminTeamMemberCandidatePreview|TestAdminPlayTeamRepairRoutesContract|TestTeamRewardSnapshotLock|TestTeamSettlementSnapshotReusesOuterTransaction)'"
run_check "FORK-PLAY-003" "daily arena reward summary tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/service ./internal/repository ./internal/handler ./migrations -run '^(TestDailyArena|TestArenaPeriodQueriesExpose|TestArenaDailyRewardSummaryRoute|TestPlayArenaDailyRewardSummaryMigrationContract)'"
run_check "FORK-IMAGE-011" "Images async, URL and Batch runtime unit tests" \
  bash -c "cd '$ROOT/backend' && go test -count=1 ./internal/repository ./internal/service ./internal/handler -run 'TestImageTask|TestImageRuntimesHealthGatewayAsync|TestAsyncImage|TestOpenAIImageResultServiceRewriteAndEnforceAPIKeyOwnership|TestOpenAIGatewayServiceForwardImages_(APIKeyStreamingURLStoresCompletedImage|OAuthStreamingTransformsEvents|StreamURLRequiresStorageBeforeUpstream)|TestBatchImage(RuntimeState|WorkerRuntime|ProviderRegistryFromConfig|PublicService_Submit)'"

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

echo
if [[ "$FAIL" -ne 0 ]]; then
  echo "Fork integrity FAILED. Review $REGISTRY before merging upstream." >&2
  exit 1
fi

echo "Fork integrity passed."
