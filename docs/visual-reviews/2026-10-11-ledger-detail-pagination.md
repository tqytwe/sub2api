# Visual Review: Request ledger detail pagination

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/requestLedger.ts",
    "frontend/src/api/__tests__/requestLedger.spec.ts",
    "frontend/src/i18n/locales/request-ledger.en.ts",
    "frontend/src/i18n/locales/request-ledger.zh.ts",
    "frontend/src/views/shared/RequestLedgerView.vue",
    "frontend/src/views/shared/components/RequestLedgerDetailSection.vue",
    "frontend/src/views/shared/__tests__/RequestLedgerView.spec.ts",
    "frontend/src/views/shared/__tests__/RequestLedgerView.session.http.spec.ts"
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
    "docs/visual-reviews/assets/ledger-detail-pagination/prototype-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/prototype-controls-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/ledger-detail-pagination/before-admin-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/ledger-detail-pagination/active-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/active-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/after-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/after-admin-1920.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/after-admin-360.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/after-admin-768.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/after-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/after-user-1920.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/after-user-360.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/after-user-768.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/attempts-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/attempts-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/dark-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/dark-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/empty-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/empty-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/en-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/en-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/error-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/error-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/focus-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/focus-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/hover-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/hover-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/loading-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/loading-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/zoom-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/zoom-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/session-account-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/session-account-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/session-logout-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/session-logout-user-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/session-renewal-admin-1280.png",
    "docs/visual-reviews/assets/ledger-detail-pagination/session-renewal-user-1280.png"
  ],
  "commands": [
    "/tmp/p0-run make build",
    "LEDGER_DETAIL_BROWSER_FIXTURE=1 LEDGER_BROWSER_DIST=/tmp/ledger-detail-built go test -tags=integration ./internal/requestledger/acceptance -run ^TestRequestLedgerDetailPagingBrowserFixture$ -count=1 -v",
    "node docs/archive/completed-tasks/ledger-detail-pagination/capture.cjs /tmp/ledger-pagination-combo-browser-fixture.log"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "note": "两身份Enter打开usage和刷新，Escape关闭；焦点截图及按钮disabled验证。"
    },
    "reduced_motion": {
      "status": "passed",
      "note": "Chromium reducedMotion=reduce，截图禁用动画；无新增动画。"
    }
  },
  "residual_risks": [
    "本地合成HTTP/PG与实际构建资源，不是生产用户本机三身份验收；本批不合并或部署。",
    "200%采用CSS zoom模拟，未声称浏览器原生缩放验收。",
    "实时分页需要主动刷新以发现新记录、迟到证明或状态变化，不提供固定快照。"
  ]
}
-->

## Scope

只调整既有请求台账详情三个集合与管理员诊断的分页交互。保持workspace页面框架、
AppLayout、BaseDialog、DataTable、Ops浮层和原样式。新组件封装页面内重复的
上一页/下一页/刷新/错误重试状态，不创建新的视觉体系。

## Baseline

实际构建资源与一次性HTTP/PG合成身份，基线与23d代码等价。原详情全量返回，
引用相邻WalletView分区分页按钮的间距、分隔线与移动换行方式；未采用需要总数
的Pagination数字页码组件，避免伪造总页数。已目视基线与原型。

## Prototype

前端修改前，以基线浏览器DOM局部插入分页控件并截图；这是静态交互原型，
不是已实现功能的证据。主线程明确批准实时游标与独立刷新边界；不引入快照生命周期。

## Reuse Decision

复用btn-secondary、语义文字颜色、既有四档视口与浮层。每页替换显示，不追加
混合旧新数据；attempt证据和对应尝试同页，Ops独立分页且仅管理员可见。

## State Coverage

组件/API22项测试已通过，覆盖独立加载/空中间页/下一页失败保留/重试/刷新/取消旧请求、usage翻页禁用及禁止旧接口回退。
实际资源双身份default/success、hover/active/focus、空中间页、延迟loading/disabled、503局部error/retry均已通过。error只在浏览器拦截单个section响应，其他数据仍由真实HTTP/PG返回。

## Viewport Coverage

已实测360/768/1280/1920双身份无页面水平溢出；1280浅深色/中英文、reduced-motion、键盘及CSS200%模拟。
目视检查360用户页、管理员错误页与深色成功页：分页按钮换行、金额/图片数与未知状态清楚，保持原浮层与表格风格。

## Evidence

基线/原型/最终PNG路径见manifest；最终交互结果见
[浏览器断言](../archive/completed-tasks/ledger-detail-pagination/browser-evidence.json)与
[可复现脚本](../archive/completed-tasks/ledger-detail-pagination/capture.cjs)。
两身份均通过legacy409、跨租户404、普通用户管理员入口403；usage真实合成结算6美元/3图与PG一致。
新UI详情读取全部明确summary/section，单页最多20条；零pageerror。游客/401由HTTP合同测试覆盖，没有登录生产。

## Residual Risk

本地合成fixture不等于生产用户本机验收；本任务不合并或部署。

## Combination revalidation

2026-10-11 09:03:20–09:03:45 UTC，普通合入#352（05b44bec）后，完整构建资源重新运行
同一双角色HTTP/PG浏览器脚本，所有断言通过，零pageerror；此页28张after/state截图来自本轮。
分页及全部前端源码不变，7个既有入口/台账/共享资源指纹不变；完整356文件资源指纹见
[组合验证](../archive/completed-tasks/ledger-detail-pagination/combination-validation.json)。
本轮重新目视普通用户360px与管理员503错误画面，样式和信息关系保持一致。


## #358 session isolation follow-up

Reused the existing dialog/loading/error/empty states and the approved prototypes; no new layout, styling or visual pattern.
The final actual build repeated both roles at all four widths with light/dark, Chinese/English and keyboard checks.
Additional lifecycle captures show normal renewal retaining attempts page 2, a new account's HTTP 503 with old data
and filters removed, and logout returning to the real login route. Six new 1280px screenshots are listed above.

The actual Pinia actions use synthetic loopback auth endpoints; ledger reads still use the real HTTP/PostgreSQL fixture.
The first lifecycle harness incorrectly expected a ledger refresh button after the production router redirected a guest
to login; correcting that assertion and rerunning both roles passed. No product change was needed.

26 real HTTP session cases plus the existing 22 view/API cases passed, including pending-publication readiness,
five-second retryable timeout, cancellation, close/reopen, independent out-of-order regions and identity isolation.
Synthetic StorageEvents do not establish every real cross-tab timing. All server screenshots remain development
evidence; production and user-local acceptance are still pending. See the session-isolation-followup JSON for times,
source/build fingerprints, gates and read-only sequential review results.


## Frozen-binding mainline combination

The fdec971d ordinary merge changes no frontend source or dependency. After the full rebuild, all 356
resource files match the reviewed session build byte-for-byte. The short actual-build HTTP/PG paging
capture was repeated for both roles and all four widths, including themes/languages, permissions,
retry/refresh and 6 USD / 3 images; zero page errors. Current standard screenshots reflect this run.
Session lifecycle screenshots/results remain from the identical-source session run. Full source proof,
gates and exported asset hashes are in ledger-detail-pagination/merge-fdec971d. No production acceptance.
