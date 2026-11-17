# 使用记录时序、延迟和连接诊断

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/components/admin/usage/UsageTable.vue", "frontend/src/components/admin/usage/UsageFilters.vue", "frontend/src/components/admin/usage/UsageApiKeyName.vue", "frontend/src/components/admin/usage/UsageDetailPopover.vue", "frontend/src/components/admin/usage/UsageTimingDialog.vue", "frontend/src/components/admin/usage/UsageConnections.vue", "frontend/src/components/admin/usage/LatencyObservabilityPanel.vue", "frontend/src/components/common/BaseDialog.vue", "frontend/src/views/admin/UsageView.vue", "frontend/src/views/admin/SettingsView.vue", "frontend/src/views/admin/ops/OpsDashboard.vue", "frontend/src/i18n/locales/en.ts", "frontend/src/i18n/locales/zh.ts", "frontend/src/i18n/locales/en/admin/settings.ts", "frontend/src/i18n/locales/zh/admin/settings.ts", "frontend/src/i18n/locales/en/dashboard.ts", "frontend/src/i18n/locales/zh/dashboard.ts", "frontend/src/i18n/locales/en/requestTiming.ts", "frontend/src/i18n/locales/zh/requestTiming.ts", "frontend/src/i18n/locales/en/connections.ts", "frontend/src/i18n/locales/zh/connections.ts", "frontend/src/i18n/locales/en/latencyObservability.ts", "frontend/src/i18n/locales/zh/latencyObservability.ts"],
  "routes_or_surfaces": ["/usage", "/admin/usage", "/admin/settings gateway usage records", "/admin/ops"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "hover", "focus-visible", "touch open", "Escape dismissal", "loading", "disabled pagination", "empty", "error", "success", "missing historic measurements", "badge hidden"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-17-usage-observability/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-17-usage-observability/baseline-1280-dark.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-17-usage-observability/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-17-usage-observability/prototype-1280-light.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-17-usage-observability/updated-table-360-light.png", "docs/visual-reviews/assets/2026-11-17-usage-observability/updated-table-1280-dark.png", "docs/visual-reviews/assets/2026-11-17-usage-observability/updated-connections-360.png", "docs/visual-reviews/assets/2026-11-17-usage-observability/updated-latency-1280.png", "docs/visual-reviews/assets/2026-11-17-usage-observability/updated-timing-360.png", "docs/visual-reviews/assets/2026-11-17-usage-observability/updated-timing-1280.png", "docs/visual-reviews/assets/2026-11-17-usage-observability/updated-token-360.png", "docs/visual-reviews/assets/2026-11-17-usage-observability/updated-controls.png"],
  "commands": ["Playwright Chromium: in-memory Vite build of actual shared UsageTable, TimingDialog, LatencyObservabilityPanel, UsageConnections and extracted SettingsView section with synthetic records and mocked API", "vitest usage component, admin UsageView, settings and cold route locale tests", "pnpm design:check", "pnpm lint:check", "pnpm typecheck"],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["Component browser evidence uses synthetic records and mocked APIs; it is not a production full-page or final user acceptance.", "Old and unsupported-protocol records show not collected; HTTP flush does not prove client receipt.", "Final production guest, user and administrator acceptance follows exact-SHA deployment."]
}
-->

## Scope

固定上游 `v2.21.2@dbed7697` 的使用记录七项缺口；不增加用户页面。管理员在既有使用记录页切换连接诊断、延迟分组，延迟列打开时序详情；运维页复用折叠报告。用户继续使用同一表格，缓存率保留 Token 列，导出和计费逻辑不变。

## Baseline

基线 `play/main@a7429fb7e`。检查既有两侧 UsageView、UsageTable、SettingsView 网关区域、OpsDashboard、DataTable 移动卡片、BaseDialog 和 HelpTooltip。基线图片先于实现生成。

## Prototype

在可见实现前，通过固定上游真实组件和现有样式生成原型图片。范围沿用用户明确要求的七项补齐；未引入 observer、流量页、网络采样、正文采集或自动协议切换。原型展示延迟与连接区，完整生产页面仍由原有布局承载。

## Reuse Decision

使用既有 AppLayout、DataTable、Icon、Select、Pagination、BaseDialog 和可访问性 composable。上游 UsageDetailPopover 对桌面按实际尺寸定位，手机使用 BaseDialog；Token／费用入口为原生按钮，长 Key 可键盘和触屏展开。BaseDialog 仅新增可选右侧布局，保留 Fork 原有焦点圈定和国际化关闭名称。

健康色条沿用现有语义图表例外：TTFT、总耗时、TPS 三段表示各自健康阈值；并非装饰性渐变。TPS 文本及定义同时给出数值，未知值不伪造为零。长上下文开关仅隐藏标记，不改变价格。

延迟分组的缓存比例沿用 Fork Token 列口径：缓存读取 /（输入 + 缓存读取 + 缓存写入）。中英文筛选标签、列标题和指标定义一致；复审已用真实语言组件断言和 360／1280px 浏览器检查重新核对并刷新延迟截图。

## State Coverage

实际组件浏览器检查默认、触屏开关、长名称、Token／费用弹窗、Enter、Escape、未知历史值、连接和报告的空数据／错误／成功，以及分页禁用。组件测试覆盖异步取消、旧响应丢弃、失败后清空旧结果、多维筛选、分页、不同身份信息和移动视口切换。设置测试覆盖保存和重载，冷路由测试防止第一次访问缺文案。

## Viewport Coverage

共享表格四个宽度 × 中英文 × 浅深色共 16 个组合；诊断区补充 360／1280px 成功、空和错误。reduced-motion 下检查，200% 缩放操作仍可见。桌面浮层按实测大小约束视口，手机切换弹窗。页面无额外横向溢出；宽延迟数据表在自身区域滚动。

## Evidence

manifest 中图片由 Chromium 渲染真实组件获得；无开发应用服务器，无生产写入或模型调用。长 Key 审查后改为复用可展开组件，避免手机单行身份块被长文本撑高。最终部署、后端和权限证据由关联 PR 汇总，当前截图不替代线上验收。

## Residual Risk

历史时序不能补采；缺少明确队列测量来源的服务端摘要显示未采集，保留原始时延及请求级实测摘要。边缘诊断只有已导入的真实记录才有入口侧证据。精确部署后仍需用户本机三身份最终验收。
