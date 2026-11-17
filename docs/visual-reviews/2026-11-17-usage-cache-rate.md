# 使用记录缓存率与平均 TPS

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/components/admin/usage/UsageTable.vue", "frontend/src/views/user/UsageView.vue", "frontend/src/views/admin/UsageView.vue", "frontend/src/views/admin/SettingsView.vue", "frontend/src/i18n/locales/zh/dashboard.ts", "frontend/src/i18n/locales/en/dashboard.ts", "frontend/src/i18n/locales/zh/admin/settings.ts", "frontend/src/i18n/locales/en/admin/settings.ts"],
  "routes_or_surfaces": ["/usage", "/admin/usage", "/admin/settings gateway usage records"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "zero cache", "no prompt tokens", "admin default", "user hidden", "feature disabled", "empty", "keyboard help", "settings save and reload", "disabled dependent switch"],
  "viewports": ["360x800", "768x800", "1280x800", "1920x800"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-17-usage-cache-rate/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-17-usage-cache-rate/baseline-1280-dark.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-17-usage-cache-rate/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-17-usage-cache-rate/prototype-1280-dark.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-17-usage-cache-rate/updated-360-light.png", "docs/visual-reviews/assets/2026-11-17-usage-cache-rate/updated-1280-dark.png", "docs/visual-reviews/assets/2026-11-17-usage-cache-rate/updated-controls.png"],
  "commands": ["Playwright Chromium compiled UsageTable and extracted actual SettingsView section, in-memory assets, no application server", "vitest usageCacheRate, UsageTable, user/admin UsageView and SettingsView", "pnpm design:check"],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["Synthetic component records are implementation evidence, not full-page production screenshots or final browser acceptance.", "Production guest/user/admin acceptance remains pending after exact SHA deployment."]
}
-->

## Scope

用户授权缓存率、两级显示开关、导出及 TPS 名称；缓存率按用户要求放在 Token 列。

## Baseline

基线 `play/main@08867026e`。先查看共享 UsageTable、两侧 UsageView、SettingsView 网关使用记录区域、DataTable、HelpTooltip 和真实组件基线截图。

## Prototype

用户于 2026-11-17 指定 Token 列位置后，位置调整前生成相应原型，见 manifest 中 prototype_artifacts。

## Reuse Decision

缓存率紧邻缓存 Token 数，点击或 Enter 查看当前三项 Token 数和公式；无有效提示 Token 显示占位，有输入但无命中显示 0%。延迟列保留首字、总耗时和平均 TPS，保留总耗时计算及服务端权威值。复用现有表格、HelpTooltip、设置 toggle 和主题样式。

## State Coverage

真实组件检查零命中、无输入、关闭、用户隐藏、空数据、Enter/Escape 和从属开关禁用。loading/error 沿用既有逻辑，hover/active/focus 复用共享控件。

## Viewport Coverage

360/768/1280/1920px，中英文、浅深色共 16 个组合无页面横向溢出，reduced-motion 环境执行。

## Evidence

截图见 manifest。挂载实际页面的测试覆盖保存刷新、身份和导出。导出开始时固定开关快照；保留 CSV 注入保护和用户字段过滤。无模型调用、开发服务器或生产设置写入。

规格审查覆盖公式、管理员默认开/用户默认关、旧客户端省略保留、保存读回和导出。代码质量审查确认仅计算展示数据，不改计费或历史数据，无需 schema migration。测试、构建、CI、精确部署证据维护在关联 PR；本机三身份浏览器验收待部署后完成，组件截图不替代最终验收。

## Residual Risk

组件与提取的真实设置区使用合成数据，不代表整页生产验收；精确部署后的用户本机验收仍待完成。
