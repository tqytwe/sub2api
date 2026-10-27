# Visual Review: Administrator-only request ledger access

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/index.ts",
    "frontend/src/router/__tests__/adminRequestLedgerAccess.spec.ts",
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/views/user/UsageView.vue",
    "frontend/src/views/user/__tests__/UsageView.spec.ts"
  ],
  "routes_or_surfaces": [
    "/usage",
    "/admin/requests"
  ],
  "languages_and_themes": [
    "zh/light"
  ],
  "states": [
    "default"
  ],
  "viewports": [
    "360x900",
    "1280x900"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/v2103-usage-tps/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/v2103-usage-tps/updated-zh-light-1920.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v2103-usage-tps/updated-zh-light-1920.png"
  ],
  "commands": [
    "node node_modules/.pnpm/vitest@2.1.9_@types+node@20.19.27_jsdom@24.1.3/node_modules/vitest/vitest.mjs run src/router/__tests__/adminRequestLedgerAccess.spec.ts src/views/user/__tests__/UsageView.spec.ts"
  ],
  "checks": {
    "access": {
      "status": "passed",
      "note": "自动化测试确认用户用量页无台账入口、普通路由不存在，管理员路由保留。"
    },
    "keyboard": {
      "status": "not-applicable",
      "reason": "No keyboard controls changed in this access-only update."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No motion or animation behavior changed in this update."
    }
  },
  "residual_risks": [
    "复用的截图是既有用户用量组件浏览器证据，不是本次改动后的完整页面截图；改动仅移除入口且不改变组件样式。",
    "最终生产验收仍需用户本机浏览器确认普通用户和管理员的可见入口。"
  ]
}
-->

## Scope

从普通用户用量页移除审计台账入口，并移除普通用户台账路由。管理员用量页入口、
`/admin/requests` 页面和普通用量记录保持原样；不涉及样式或组件变更。

## Baseline

复用现有用户用量浏览器原型和组件截图。页面框架、统计卡片和用量表格没有变化；
本次差异是移除通往管理员审计能力的按钮及对应普通用户路由。

## Prototype

复用既有用户用量原型图作为保留页面内容的视觉基准；本次无新控件和样式设计，
原型边界是保留用量页面并移除审计入口。

## Reuse Decision

未新增视觉组件、样式或状态。保留管理员已有入口及共享台账视图；普通用户继续使用
原用量记录页，只是不再提供审计台账链接。

## State Coverage

检查 default 状态；hover、active、focus-visible、loading、disabled、empty、error、
success 均不受本次改动影响，因为页面和组件状态逻辑没有变化。

## Viewport Coverage

复用既有 1920px 用户用量组件截图；本次只移除链接，不改变页面布局或组件响应式规则。

## Evidence

路由测试确认 `/requests` 不再注册、`/admin/requests` 仍带管理员权限；用户用量页测试
确认审计台账标题和 `/requests` 链接不存在。截图是此前生成的组件证据，不冒充本次修改后
整页截图。

## Residual Risk

最终生产验收仍需用户本机浏览器确认普通用户和管理员的可见入口。当前证据不替代生产验收。
