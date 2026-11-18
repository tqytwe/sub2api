# 上游渠道与利润报表视觉审查

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/index.ts",
    "frontend/src/api/admin/profitOperations.ts",
    "frontend/src/api/admin/upstreamChannels.ts",
    "frontend/src/components/account/AccountUpstreamChannelBinding.vue",
    "frontend/src/components/account/UpstreamChannelSelector.vue",
    "frontend/src/components/account/upstreamChannelCreation.ts",
    "frontend/src/components/account/__tests__/UpstreamChannelBinding.spec.ts",
    "frontend/src/components/admin/operations/ProfitAnalytics.vue",
    "frontend/src/components/admin/operations/UpstreamChannelManager.vue",
    "frontend/src/components/admin/operations/__tests__/UpstreamChannelManager.spec.ts",
    "frontend/src/i18n/locales/en/profitOps.ts",
    "frontend/src/i18n/locales/en/upstreamEntities.ts",
    "frontend/src/i18n/locales/zh/profitOps.ts",
    "frontend/src/i18n/locales/zh/upstreamEntities.ts",
    "frontend/src/components/account/BulkEditAccountModal.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/types/index.ts",
    "frontend/src/views/admin/ChannelOperationsView.vue"
  ],
  "routes_or_surfaces": [
    "/admin/channels/operations",
    "/admin/accounts"
  ],
  "languages_and_themes": [
    "zh/light",
    "en/dark"
  ],
  "states": [
    "default",
    "focus-visible",
    "disabled",
    "loading",
    "empty",
    "error"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "commands": [
    "Playwright Chromium browser screenshots of compiled Vue components with fixture API responses",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "账号表单和真实生产数据仍需部署后在管理员浏览器中验收。"
  ],
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-19-channel-profit/prototype-1280-light-default.png",
    "docs/visual-reviews/assets/2026-11-19-channel-profit/prototype-360-light-binding.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-19-channel-profit/baseline-1280-light-default.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-1280-light-default.png",
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-1280-light-stateempty.png",
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-1280-light-stateloading.png",
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-1920-dark-default.png",
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-360-light-binding.png",
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-360-light-default.png",
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-768-dark-default.png",
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-768-dark-stateerror.png"
  ]
}
-->

## Scope

管理员在现有渠道运维查看利润并管理上游渠道，在账号表单设置归属。

## Baseline

使用 #435 的渠道运维组件检查原有成员和统计入口。

## Prototype

开发前编译固定上游的报表和绑定组件，使用现有样式捕获原型。

## Reuse Decision

复用后台布局、页签、表单和按钮。账号编辑和批量编辑使用同一归属组件。

## State Coverage

检查正常、加载、空表、错误和禁用状态。通过键盘打开绑定控件和利润下钻。

## Viewport Coverage

以 360、768、1280 和 1920 像素宽度检查中英文及浅色、深色界面。四种宽度无页面横向溢出，启用减少动态效果。

## Evidence

清单中的截图来自 Chromium 运行编译后的 Vue 组件，查询使用合成响应。

## Residual Risk

完整账号表单与真实生产数据需要部署后验收。
