# 请求台账视觉审查

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/requestLedger.ts",
    "frontend/src/i18n/locales/request-ledger.en.ts",
    "frontend/src/i18n/locales/request-ledger.zh.ts",
    "frontend/src/views/shared/RequestLedgerView.vue",
    "frontend/src/views/shared/__tests__/RequestLedgerView.spec.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/core.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/core.ts",
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/router/index.ts",
    "frontend/src/views/admin/UsageView.vue",
    "frontend/src/views/user/UsageView.vue"
  ],
  "routes_or_surfaces": [
    "/requests: authenticated workspace, fluid frame, normal density, shared header, document scroll and shared background",
    "/admin/requests: admin workspace, fluid frame, normal density, shared header, document scroll and shared background",
    "Usage pages: request ledger navigation button"
  ],
  "languages_and_themes": [
    "zh-CN light",
    "zh-CN dark",
    "en-US light"
  ],
  "states": [
    "default with known/unknown usage, pending/settled billing and all execution states",
    "detail, existing wallet reference and original usage drilldown",
    "empty filtered result",
    "browser injected read failure, then retry against real API",
    "loading and disabled shared controls",
    "admin pagination and language-preserving filters"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/request-ledger/prototype-admin-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/request-ledger/baseline-admin-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/request-ledger/dark-admin-1280.png",
    "docs/visual-reviews/assets/request-ledger/dark-user-1280.png",
    "docs/visual-reviews/assets/request-ledger/detail-admin-1280.png",
    "docs/visual-reviews/assets/request-ledger/detail-user-1280.png",
    "docs/visual-reviews/assets/request-ledger/error-admin-1280.png",
    "docs/visual-reviews/assets/request-ledger/error-user-1280.png",
    "docs/visual-reviews/assets/request-ledger/loading-admin-1280.png",
    "docs/visual-reviews/assets/request-ledger/loading-user-1280.png",
    "docs/visual-reviews/assets/request-ledger/updated-admin-1280.png",
    "docs/visual-reviews/assets/request-ledger/updated-admin-1920.png",
    "docs/visual-reviews/assets/request-ledger/updated-admin-360.png",
    "docs/visual-reviews/assets/request-ledger/updated-admin-768.png",
    "docs/visual-reviews/assets/request-ledger/updated-admin-en-1280.png",
    "docs/visual-reviews/assets/request-ledger/updated-empty-1280.png",
    "docs/visual-reviews/assets/request-ledger/updated-user-1280.png",
    "docs/visual-reviews/assets/request-ledger/updated-user-1920.png",
    "docs/visual-reviews/assets/request-ledger/updated-user-360.png",
    "docs/visual-reviews/assets/request-ledger/updated-user-768.png",
    "docs/visual-reviews/assets/request-ledger/updated-user-en-1280.png",
    "docs/visual-reviews/assets/request-ledger/usage-admin-en-1280.png",
    "docs/visual-reviews/assets/request-ledger/usage-user-en-1280.png"
  ],
  "commands": [
    "node /tmp/ledger-visual/capture.cjs: Playwright browser baseline and prototype",
    "node /tmp/ledger-visual/verify.cjs: Playwright real PostgreSQL HTTP/API fixture refresh, detail, themes and responsive screenshots",
    "node /tmp/ledger-visual/states.cjs: Playwright keyboard Enter/Escape, reduced motion and empty filter",
    "pnpm exec vitest run src/views/shared/__tests__/RequestLedgerView.spec.ts",
    "pnpm typecheck",
    "node /tmp/ledger-visual/final-states.cjs: English filters, pagination, usage drilldown, loading, synthetic read error and real API retry"
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
    "Local disposable PostgreSQL and synthetic identities are development evidence; user local browser production acceptance remains with the root delivery thread.",
    "Browser failure injection covers UI read errors, not a claim that all provider failures were exercised."
  ]
}
-->

## 范围与复用

沿用现有用户/管理用量页入口、AppLayout、DataTable、Select、Pagination、BaseDialog 和 Icon。新页面不设置私有宽度、页面滚动或背景。原型是在浏览器中依照现有框架渲染的审查图；最终图由生产构建连接本地一次性 PG 的真实 API 截取。用户已明确授权保持现有风格的台账页面，边界仅为只读台账及用量关联。

## 状态与证据

实际验证普通用户与管理员刷新、详情、浅深色、四档视口且无横向溢出；键盘 Enter 打开、Escape 关闭；reduce 媒体设置下可用；真实过滤无结果。英文筛选保留语言、管理员第二页、既有钱包引用与 usage 原记录钻取均已浏览器验证，7/3 token 和 $1.2500 来自现有结算仓库写入的隔离 PG。加载状态由暂缓只读响应观察；错误状态由浏览器注入 503，解除注入后重试真实 API 成功。两种角色无 pageerror。组件测试覆盖未知消费不渲染为 $0。无删除或收费操作，没有这类确认/成功状态。筛选按钮与列表 loading 采用共享组件禁用状态。

## 剩余检查

最终 production assets 于 06:45–06:46 UTC 复验并刷新截图：页头余额 $98.75 来自同一 PG，原始余额 $100 减既有结算 $1.25；usage 的 7/3 token 与 $1.2500 钻取一致。两角色无 pageerror，视觉已复核。完整本地门禁见最终审查记录；本地截图不能替代根线程发布后的用户本地生产验收。

## Scope
用户与管理员请求台账，目标 route contract 见 manifest。

## Baseline
实际基线见 baseline-admin-1280.png；原页面入口和布局已检查。

## Prototype
实际浏览器原型见 prototype-admin-1280.png；只读台账边界按用户授权实施。

## Reuse Decision
见上文复用记录，未新增共享视觉模式。

## State Coverage
见上文状态记录及 manifest；未完成项目如实列在剩余检查。

## Viewport Coverage
360 / 768 / 1280 / 1920，实际浏览器无横向溢出。

## Evidence
manifest 中的真实 PNG 和命令对应本地合成身份、一次性 PG 的验证，不代表生产验收。

## Residual Risk
生产发布及用户本地电脑最终验收由根线程统一处理；本地合成身份不代替生产验收。
