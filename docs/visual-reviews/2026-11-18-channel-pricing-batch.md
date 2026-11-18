# 渠道价格批量管理视觉审查

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/ChannelsView.vue",
    "frontend/src/components/admin/channel/BulkEditChannelDialog.vue",
    "frontend/src/i18n/locales/en/admin/channels.ts",
    "frontend/src/i18n/locales/zh/admin/channels.ts",
    "frontend/src/views/admin/channelPricingBundle.ts",
    "frontend/src/views/admin/channelPricingTransfer.ts"
  ],
  "routes_or_surfaces": [
    "/admin/channels/pricing",
    "channel pricing bulk review dialog"
  ],
  "languages_and_themes": [
    "zh/light",
    "en/dark"
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
    "success",
    "import draft",
    "200 percent CSS zoom"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/prototype-1280.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/prototype-dialog-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/baseline-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-360-zh-light.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-pricing-360-zh-light.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-768-en-dark.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-pricing-768-en-dark.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-1280-zh-light.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-pricing-1280-zh-light.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-1920-en-dark.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-pricing-1920-en-dark.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-loading-1280.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-error-1280.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-success-1280.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-import-draft-1280.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-empty-360.png",
    "docs/visual-reviews/assets/2026-11-18-channel-pricing-batch/updated-css-zoom-200.png"
  ],
  "commands": [
    "Playwright browser screenshot: node docs/archive/completed-tasks/2026-11-18-channel-pricing-batch/capture.mjs",
    "pnpm design:check",
    "pnpm lint:check",
    "pnpm typecheck"
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
    "The first implementation preceded prototype capture; evidence records the actual sequence rather than claiming compliance retroactively.",
    "In-memory fixture records and server Chromium captures are implementation evidence; exact deployment and user local guest/user/admin acceptance remain pending.",
    "CSS 200 percent zoom is a development proxy; final native browser zoom acceptance remains pending."
  ]
}
-->

## Scope

仅第17项 `/admin/channels/pricing` 管理员渠道价格批量管理。新增多选、批量修改、价格包导入导出和导入审阅；不涉及上游厂商/利润、第18项、账单结算或数据库迁移。路由继续复用现有 app/workspace frame。

## Baseline

基线为生产 #432 `6385c821143b2b8141bdf08106f4390b7fbace4d` 的真实 ChannelsView，通过同一共享布局和样例数据编译捕获 `baseline-1280.png`。查看 ChannelsView、相邻 GroupsView、AppLayout、TablePageLayout、DataTable、BaseDialog、PricingEntryCard 和设计规范。原页面只有逐渠道 CRUD，不提供多选价格管理。

## Prototype

原型为真实组件的 `prototype-1280.png` 与 `prototype-dialog-1280.png`。必须如实记录：首轮实现早于原型保存，未满足设计规范的先原型流程；补图不能追溯消除此缺口。用户任务边界已由父代理确认是第17项；恢复后先保存基线/原型，才继续错误修复和可见功能补全。原型明确复用现有管理页布局、表格和弹窗，不引入新视觉体系。

## Reuse Decision

复用 AppLayout、TablePageLayout、DataTable 的桌面表格/移动卡片选择、BaseDialog 的焦点圈定/Escape、ConfirmDialog、Select、Toggle、PricingEntryCard、Icon、按钮和语义色。新文件只组合现有组件，不新增共享样式模式、页面宽度或SVG。价格模板未包含的平台保留；空平台明确提示将清空。

## State Coverage

默认/禁用：无选择时批量编辑与选中导入禁用，本页导出可用；选中后显示目标数量。hover/active 复用按钮状态，无布局变化。键盘在四个视口执行 Tab、Escape 与返回触发按钮检查。loading 设置 aria-busy、禁止重复提交与关闭；失败保留全部价格目标并显示原因/重试；成功才关闭。导入只打开待保存草稿，捕获前后写请求数量不变。另捕获空表、长模型名、CSS 200%缩放。

## Viewport Coverage

360px 中文浅色、768px 英文深色、1280px 中文浅色、1920px 英文深色：均无文档横向溢出，操作可达；移动端复用 DataTable 卡片布局。长模型名在既有 PricingEntryCard 折叠摘要中截断，展开可编辑完整值。reduced-motion 在所有捕获中启用。200%采用CSS zoom作为开发检查，不冒充系统浏览器缩放验收。

## Evidence

14张最终PNG、执行断言与模拟写请求见 `assets/2026-11-18-channel-pricing-batch/updated-browser.json`。capture.mjs 编译真实页面及共享组件，Playwright内存路由提供合成数据，不启动服务器、Vite dev或生产API访问；模拟写入仅在浏览器夹具内处理。浏览器零pageerror，四视口焦点返回、加载冻结、失败重试、导入不写入均通过。

## Residual Risk

首轮原型顺序缺口已记录，不能宣称完全遵守前置设计流程。服务器截图只能支持实现审查；生产精确SHA部署后仍需用户本地电脑三身份、主题、原生200%缩放验收。当前未推送、合并或部署。
