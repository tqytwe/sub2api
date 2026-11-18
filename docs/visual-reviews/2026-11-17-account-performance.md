# 账号性能、阈值暂停和统计窗口

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/AccountsView.vue", "frontend/src/views/admin/SettingsView.vue", "frontend/src/components/account/AccountPerformanceCell.vue", "frontend/src/components/account/AccountPerformanceDialog.vue", "frontend/src/components/settings/AccountPerformanceSettings.vue", "frontend/src/api/admin/accounts.ts", "frontend/src/api/admin/settings.ts", "frontend/src/i18n/locales/en.ts", "frontend/src/i18n/locales/zh.ts", "frontend/src/i18n/locales/en/accountPerformance.ts", "frontend/src/i18n/locales/zh/accountPerformance.ts", "frontend/src/utils/opsMetricThresholds.ts"],
  "routes_or_surfaces": ["/admin/accounts performance column and threshold dialog", "/admin/settings gateway account performance window"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "hover", "active", "focus-visible", "loading", "disabled", "empty", "error", "success", "paused", "partial HTTP history", "conflicting revision"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-17-account-performance/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-17-account-performance/baseline-1280-dark.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-17-account-performance/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-17-account-performance/prototype-dialog-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-17-account-performance/updated-360-light.png", "docs/visual-reviews/assets/2026-11-17-account-performance/updated-1280-dark.png", "docs/visual-reviews/assets/2026-11-17-account-performance/updated-dialog-360.png", "docs/visual-reviews/assets/2026-11-17-account-performance/updated-dialog-1280.png", "docs/visual-reviews/assets/2026-11-17-account-performance/updated-keyboard.png"],
  "commands": ["Playwright Chromium browser capture: node /data/tmp/v2212-account-performance-visual.mjs baseline", "Playwright Chromium browser capture: node /data/tmp/v2212-account-performance-visual.mjs prototype", "Playwright Chromium browser capture: node /data/tmp/v2212-account-performance-visual.mjs updated", "vitest account performance API, cell, dialog and settings tests"],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["Actual shared components with synthetic account data and mocked API responses; these are development component captures, not full production page screenshots.", "Final production guest/user/admin browser acceptance remains pending after integration and exact SHA deployment."]
}
-->

## Scope

按固定上游 v2.21.2/dbed7697 移植管理员账号性能列、阈值编辑/释放/事件及全局统计窗口。新增功能留在现有账号表和设置网关 tab，不增页面或用户入口。现有 AppLayout、TablePageLayout 和账号列表功能保留。

## Baseline

基线为 play/main@6385c821。实现前阅读 AccountsView、SettingsView 网关配置区、相邻 AccountCapacityCell、共享 DataTable、HelpTooltip、BaseDialog、按钮及现有主题规则。基线图片使用当前真实共享表格渲染现有名称/容量/状态；不冒充已登录生产整页。

## Prototype

先使用固定上游组件和 Fork 共享组件实际渲染并保存原型。用户授权边界是同步这项完整上游功能。仅移除本 Fork 不存在的新功能通知系统依赖；帮助图标改用已有 Icon，保留上游统计、阈值、解释及事件布局。原型先于本次前端可见实现生成。

## Reuse Decision

复用 DataTable 的桌面/手机行、BaseDialog 的宽屏与手机浮层、HelpTooltip 点击说明、Icon 的 infoCircle、既有 input/btn、暗色主题和 latencyHealth 语义配色。设置使用现有 card 及网关 tab 区域。没有新增私有页面宽度、导航、图标体系或图表。

## State Coverage

组件测试覆盖未采集与 API 失败的区分、真实零值、部分进程 HTTP 历史、上下界校验、旧配置兼容、版本冲突保留编辑、独立暂停释放、加载失败重试、保存成功以及保存中的禁用。浏览器检查实际按钮 Enter 打开、Space 切换、Esc 关闭并恢复表格操作、阈值输入和无样本/错误文本。hover/active/focus 沿用共享控件样式，无新增动画。

## Viewport Coverage

360、768、1280、1920px × 中英文 × 浅深色共 16 个组合无页面横向溢出；360/1280px 弹窗实际截图确认阈值输入和底部操作可达，正文在共享浮层内滚动。浏览器使用 reduced-motion。200% 页面缩放尚需生产整页验收补查，不能把组件截图当最终证据。

## Evidence

截图为 Chromium 执行实际 Vue 组件生成的 PNG。使用合成账号及内存 API stub，在浏览器拦截域加载内存构建资源，没有启动应用开发服务器、模型请求或生产设置写入。每组脚本检查 pageerror 及横向溢出。API 与组件 4 组 48 项测试、typecheck、改动文件 ESLint 已通过；最终全套门禁与部署记录由累计集成批次维护。

## Residual Risk

最终需在精确提交部署后由用户本地浏览器验收真实管理员列表、配置持久化、事件与窗口；同时回归游客/普通用户权限。组件级合成数据和截图只证明实现及交互，不证明生产指标、部署成功或全页无回归。
