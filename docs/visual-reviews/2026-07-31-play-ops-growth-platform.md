# Visual Review: play operations growth platform

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/play.ts",
    "frontend/src/api/play.ts",
    "frontend/src/api/referralCampaign.ts",
    "frontend/src/components/admin/play/AdminAppAnalyticsOperations.vue",
    "frontend/src/components/admin/play/AdminInviteGrowthOperations.vue",
    "frontend/src/components/admin/play/AdminMembershipOperations.vue",
    "frontend/src/components/admin/play/AdminQuizQuestionBank.vue",
    "frontend/public/downloads/android-version.json",
    "frontend/public/downloads/jisudengchat-android.apk",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/views/admin/PlayOpsView.vue",
    "frontend/src/views/admin/__tests__/PlayOpsView.spec.ts",
    "frontend/src/views/auth/EmailVerifyView.vue",
    "frontend/src/views/auth/OAuthCallbackView.vue",
    "frontend/src/views/auth/RegisterView.vue",
    "frontend/src/views/public/AndroidDownloadView.vue",
    "frontend/src/views/public/AgentTeamView.vue",
    "frontend/src/views/user/PlayHubView.vue",
    "frontend/src/views/user/AffiliateView.vue"
  ],
  "routes_or_surfaces": ["/admin/play-ops", "/play", "/affiliate", "/register", "/email-verify", "/download/android", "OAuth registration callbacks"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "disabled", "empty", "error", "success", "hover", "focus-visible"],
  "viewports": ["360x800", "390x844", "768x900", "1280x900", "1600x1000"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"
  ],
  "commands": [
    "pnpm design:check",
    "pnpm lint:check",
    "pnpm typecheck",
    "pnpm test"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Tabs expose tab roles and selected state; filters and actions use native controls with focus-visible styles." },
    "reduced_motion": { "status": "passed", "notes": "No continuous or layout motion was introduced." },
    "copy_locale": { "status": "passed", "notes": "New system labels use paired Chinese and English locale keys; dynamic campaign and account content remains server data." }
  },
  "residual_risks": [
    "The prototype is a static review board and must be replaced or supplemented with rendered implementation screenshots.",
    "Final product acceptance remains required in the production browser and Android build."
  ]
}
-->

## Scope

在现有玩法运营入口内引入按业务域切换的 TAB，并补齐会员、活动、邀请增长、团队、APP 数据和反馈管理。用户端继续复用现有 `/play`、邀请与团队入口，不新增顶级导航或平行设计体系。

## Baseline

当前页面将多个大型运营区域同时展开并同时加载，反馈仅有局部折叠。已有页面使用 `AppLayout`、现有卡片、表格、按钮、表单、Toast 和 `growth-world.css`。

## Prototype

- Prototype design image: `docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png`
- Prototype source: `docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.html`
- Approval status: 用户已确认会员、定向活动、邀请增长、双二维码、APP 统计、反馈整改及 TAB 化完整方案并要求进入开发。
- Scope boundary: 保持 `/admin/play-ops`、`/play`、`/affiliate` 与 `/agent-team` 既有入口；只在既有布局和设计令牌内重组与补齐功能。

## Reuse Decision

- 复用 `AppLayout`、`AppSidebar`、`AppHeader`、现有 Button/Input/Select/Dialog/Table/Toast 和 `Icon.vue`。
- 运营页面保持 8px 卡片、6px 控件、语义颜色和紧凑数据密度。
- TAB 使用既有导航/分段选择视觉语言，并以 URL 查询参数保存活动状态。
- 不新增页面级宽度、居中容器、全屏背景、装饰渐变或卡片嵌套。

## State Coverage

- Default: 当前 TAB 的统计、筛选、表格和操作。
- Loading: 仅当前 TAB 显示区域加载状态，其他 TAB 不发请求。
- Empty: 区分无数据、筛选无结果和无权限。
- Error: 保留筛选和草稿，显示本地化原因与重试操作。
- Success: 保存、审批、领取和状态修改后读取持久化结果并就地确认。
- Disabled: 未满足发布、审批、预算或领取条件时说明原因。
- Hover/focus-visible: 不发生布局位移，所有 TAB、筛选、图标按钮和行操作可键盘到达。

## Viewport Coverage

- 360x800、390x844: TAB 横向可滚动，筛选纵向排列，邀请明细使用移动卡片/展开详情，关键按钮不溢出。
- 768x900: 指标与筛选自动换行，主次区域顺序保持一致。
- 1280x900 及以上: 使用侧栏右侧全部可用宽度，表格与详情按现有工作区模式布局。
- 200% zoom: 操作保持可达，数据表提供明确溢出容器，不让页面根节点产生横向滚动。
- reduced motion: 不使用持续动画、位移或脉冲。

## Evidence

- Pre-implementation static prototype: `docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png`.
- Updated browser captures: pending implementation.
- Automated overlap, locale and state checks: pending implementation.

## Residual Risk

- 当前原型仅用于确认信息架构和视觉边界，不代表浏览器实现证据。
- 完成代码后必须补充中英文、浅深色和移动/桌面截图，并由用户在生产环境完成最终验收。
