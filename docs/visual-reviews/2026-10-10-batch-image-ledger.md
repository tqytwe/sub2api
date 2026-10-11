# Visual Review: Batch image ledger usage drill

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/requestLedger.ts",
    "frontend/src/i18n/locales/request-ledger.en.ts",
    "frontend/src/i18n/locales/request-ledger.zh.ts",
    "frontend/src/views/shared/RequestLedgerView.vue",
    "frontend/src/views/shared/__tests__/RequestLedgerView.spec.ts"
  ],
  "routes_or_surfaces": [
    "/requests",
    "/admin/requests"
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
    "docs/visual-reviews/assets/batch-image-ledger/prototype-admin-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/batch-image-ledger/before-admin-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/batch-image-ledger/after-admin-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/after-admin-1920.png",
    "docs/visual-reviews/assets/batch-image-ledger/after-admin-360.png",
    "docs/visual-reviews/assets/batch-image-ledger/after-admin-768.png",
    "docs/visual-reviews/assets/batch-image-ledger/after-user-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/after-user-1920.png",
    "docs/visual-reviews/assets/batch-image-ledger/after-user-360.png",
    "docs/visual-reviews/assets/batch-image-ledger/after-user-768.png",
    "docs/visual-reviews/assets/batch-image-ledger/dark-admin-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/dark-user-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/en-admin-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/en-user-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/error-admin-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/error-user-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/loading-admin-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/loading-user-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/unknown-count-admin-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/unknown-count-user-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/zoom-admin-1280.png",
    "docs/visual-reviews/assets/batch-image-ledger/zoom-user-1280.png"
  ],
  "commands": [
    "pnpm --dir frontend build",
    "LEDGER_BATCH_BROWSER_FIXTURE=1 LEDGER_BROWSER_DIST=/tmp/ledger-batch-built go test -tags=integration ./internal/requestledger/acceptance -run ^TestBatchImageBrowserFixture$",
    "PLAYWRIGHT_MODULE=... node docs/archive/completed-tasks/batch-image-ledger/capture.cjs /workspace/batch-ledger-browser-combination.log"
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
    "真实构建资源、隔离 HTTP/PG 和合成身份；不是生产浏览器验收。",
    "200% 是 CSS zoom 模拟，不宣称操作系统或原生浏览器缩放验收。",
    "503/loading/nullable image_count 通过浏览器测试拦截模拟；后端故障与权限另有真实 PG/HTTP 覆盖。"
  ]
}
-->

## Scope

沿用 `/requests` 与 `/admin/requests` 的 workspace frame、AppLayout、BaseDialog、
DataTable、按钮、语义颜色与双语字段。未新增路由、全局布局或视觉模式。

## Baseline

基线来自 9b367c 构建资源与一次性数据库；当时 capture 已结算，但不能钻取其 usage。

## Prototype

修改可见代码前，先用原有详情 DOM 生成原型图，明确只在计费关联展示共享批次文案、
图片数及 batch ID。原型是浏览器内局部文字 mockup，不是功能完成证据；在现有授权范围
内完成设计边界。最终图片来自本分支完整构建资源与真实 HTTP/PG 联动。
## Reuse Decision

继续复用详情两列网格、移动端单列和 break-all，不引入自绘组件或例外样式。

## State Coverage

- Default/success：两种身份均显示 3 张、$6、同一批次；共享结算不声称每个请求各扣一次。
- Hover/active：复用原按钮样式；脚本 click 覆盖操作，无新增专属状态样式。
- Focus-visible/keyboard：Tab 可用的原 usage 按钮获焦点后 Enter 打开；Escape 关闭。
- Loading/error/retry：测试拦截模拟延迟与 503，保留原 status/alert，重试后恢复真实响应。
- Empty/disabled：本次未新增相应交互；沿用现有台账空态/禁用条件，组件测试覆盖既有状态。
- Unknown：模拟 nullable count，明确“Unknown usage”；图片模式隐藏 token 对，token 模式测试保持。

## Viewport Coverage

- 360/768/1280/1920 × 两身份：无 document 横向溢出；长 batch ID 在小屏换行。
- 1280 两身份深色和英文截图；reduced-motion 浏览器上下文；CSS 200% zoom 截图仅供观察，未断言全部详情在该模拟模式下可见。

## Evidence

[结构化浏览器结果](../archive/completed-tasks/batch-image-ledger/browser-evidence.json)
记录请求/批次/usage 精确标识。2026-10-11 在 c939d570 组合基线完成全量构建后重拍全部 after/state 图；原 before/prototype 图保留。真实 items HTTP 图片数合计为 3，与 usage 相同，扣费为 6；
两身份均无 pageerror。已目视 360 用户、1280 管理员最终图及规格审查记录的深色图。

## Residual Risk

这些是开发技术验证。未访问生产、未调用图片供应商。最终生产游客/用户/管理员验收由
主线程获准部署后协调，不能用此处 localhost 证据替代。
