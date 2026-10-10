# Visual Review: P0 ledger exact correlation

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/ops.ts",
    "frontend/src/api/requestLedger.ts",
    "frontend/src/i18n/__tests__/coldRouteLocaleScopes.spec.ts",
    "frontend/src/i18n/locales/request-ledger.en.ts",
    "frontend/src/i18n/locales/request-ledger.zh.ts",
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue",
    "frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts",
    "frontend/src/views/shared/RequestLedgerView.vue",
    "frontend/src/views/shared/__tests__/RequestLedgerView.spec.ts"
  ],
  "routes_or_surfaces": [
    "/requests",
    "/admin/requests",
    "OpsErrorDetailModal"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en/light"
  ],
  "states": [
    "default",
    "hover",
    "active",
    "focus-visible",
    "loading",
    "disabled",
    "empty",
    "error",
    "success"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/p0-ledger-correlation/prototype-admin.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/p0-ledger-correlation/before-admin-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/p0-ledger-correlation/after-admin-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/after-admin-1920.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/after-admin-360.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/after-admin-768.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/after-user-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/after-user-1920.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/after-user-360.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/after-user-768.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/dark-admin-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/dark-user-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/en-admin-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/en-user-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/error-admin-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/error-user-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/ops-admin-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/evidence-unavailable-admin-1280.png",
    "docs/visual-reviews/assets/p0-ledger-correlation/evidence-unavailable-user-1280.png"
  ],
  "commands": [
    "pnpm --dir frontend build",
    "LEDGER_BROWSER_FIXTURE=1 go test -tags=integration ./internal/requestledger/acceptance -run ^TestRequestLedgerBrowserFixture$",
    "Playwright browser: node p0-capture.cjs (archived under completed-tasks/p0-ledger-correlation/)"
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
    "技术截图来自实际production assets和一次性PG/HTTP fixture，不能替代生产三身份验收。",
    "旧记录无证据/读取失败/重试由自动测试覆盖；未执行供应商自然生成请求。",
    "200%缩放未单独截图；已测360px响应式布局。"
  ]
}
-->

## Scope

用户与管理员现有台账详情新增 exact evidence；管理员复用 OpsErrorDetailModal 双向精确跳转。route contract 保持 workspace frame。无全站重设计。

## Baseline

原详情只能查看尝试/用量/计费，无法从 ledger 精确打开 Ops。before-admin-1280.png 来自基线构建资源；原型截图在实现 UI 前生成。

## Prototype

prototype-admin.png 先展示新增证据 section、attempt UUID 与管理员 Ops 入口。用户已授权必要展示与跳转一致性；本次按原布局实现，不扩展视觉风格。

## Reuse Decision

复用 AppLayout、BaseDialog、DataTable、Pagination、Select、btn 与原 Ops 弹窗，无新共享视觉模式或设计例外。UUID 使用 break-all。浏览器发现并修复冷路由 Ops 文案未加载及错误 phase 假设。

## State Coverage

- default/success：实际 GET 返回 UUID 与 PG/API 匹配，两次 attempt 不同；HTTP200 failure、unknown、pending 仍分开。
- hover/active/focus-visible：复用原按钮；聚焦 Ops 按钮并 Enter 打开，再点击精确反链；Escape 关闭等待过渡完成。
- loading：原 aria-busy/status，组件测试与实际请求加载经过，未独立保留截图。
- disabled：不可用 Ops 为静态文本，不创建可点击控件；截图含原诊断删除状态。
- empty：无证据/无计费保持原空态与不确定说明，组件测试覆盖；无 $0 推算。
- error：拦截单次详情 API 返回503，截图展示错误与 Retry；移除注入后恢复真实 API，截图 error-*.png。

## Viewport Coverage

360/768/1280/1920 × 用户与管理员均截图，文档横向无溢出。中英与浅/深色截图检查；英文宽表沿用共享表格内部横向滚动，页面不溢出。reduced-motion 浏览器上下文启用。200% zoom 未单独验证，不声称覆盖。

## Evidence

本目录 PNG 为 Chromium 实际构建资源截图，全部数据为 synthetic fixture。browser-evidence.json 保存角色、API匹配UUID、视口、键盘、页面错误列表（空）。脚本归档可复跑（PLAYWRIGHT_MODULE 可指定 Playwright 模块，CHROMIUM_BIN 可指定 Chromium），使用当前 fixture 端口18762；先启动 acceptance fixture，构建并复制静态输出到它指定的路径。

## Residual Risk

无生产凭据、生产账户或生成调用。游客访问由实际HTTP 401测试覆盖，未做生产游客浏览器验收。生产部署与用户本机验收仍由主线程协调。此截图不证明历史请求可追溯或历史损失恢复。


## 草稿复审增量

沿用原型中的证据section、现有琥珀色状态文案和布局。证据表读取失败新增明确状态，保持核心台账可见；Ops弹窗的代际修复保持布局不变。同路由反向跳转回归保留。本轮已重新执行 Chromium 捕获：实际构建的用户/管理员页面、四视口、中英/主题、键盘、Ops 精确反链及失败重试通过。新增两张 evidence-unavailable 截图由浏览器对真实隔离 fixture 响应注入可选证据读取失败标志，确认核心台账仍可见；数据库缺表/锁表由独立真实 PG/HTTP 测试验证。build-assets.json 记录本轮资源 SHA256，均不是生产验收。
