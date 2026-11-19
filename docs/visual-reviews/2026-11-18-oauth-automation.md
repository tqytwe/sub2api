# OAuth 自动配置与降级控制面板

<!-- visual-review-manifest
{
 "schema_version": 1,
 "changed_files": ["frontend/src/views/admin/AccountQualityView.vue", "frontend/src/components/admin/operations/OAuthAutomationPanel.vue", "frontend/src/components/admin/operations/OAuthDegradationCard.vue", "frontend/src/components/admin/operations/OpenAIAPIKeyFallbackCard.vue", "frontend/src/components/admin/operations/OAuthInitialModelSelection.vue", "frontend/src/components/admin/operations/QualityAccountSelector.vue", "frontend/src/components/admin/operations/AutoConfigHistory.vue", "frontend/src/api/admin/autoConfig.ts", "frontend/src/utils/oauthDegradation.ts", "frontend/src/utils/openaiAPIKeyFallback.ts", "frontend/src/utils/oauthModelMappings.ts", "frontend/src/utils/qualityAccountFilter.ts", "frontend/src/i18n/index.ts", "frontend/src/i18n/locales/en.ts", "frontend/src/i18n/locales/zh.ts", "frontend/src/i18n/locales/en/autoConfig.ts", "frontend/src/i18n/locales/zh/autoConfig.ts"],
 "routes_or_surfaces": ["/admin/account-quality: existing administrator quality operations page"],
 "artifact_mode": "browser-capture",
 "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
 "states": ["default", "hover", "active", "focus-visible", "loading", "disabled", "empty", "error", "success", "unavailable selected accounts", "pending history page", "stale draft", "collapsed"],
 "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
 "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-18-oauth-automation/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-18-oauth-automation/baseline-1280-dark.png"],
 "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-18-oauth-automation/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-18-oauth-automation/prototype-1280-dark.png"],
 "updated_artifacts": ["docs/visual-reviews/assets/2026-11-18-oauth-automation/updated-360-light.png", "docs/visual-reviews/assets/2026-11-18-oauth-automation/updated-1280-dark.png", "docs/visual-reviews/assets/2026-11-18-oauth-automation/updated-keyboard.png"],
 "commands": ["Playwright browser capture: node /data/tmp/task10-visual.mjs baseline", "Playwright browser capture: node /data/tmp/task10-visual.mjs prototype", "Playwright browser capture: node /data/tmp/task10-visual.mjs updated", "vitest run OAuth automation controls/history/inventory/cold locale and existing AccountQualityView tests", "eslint task10 changed components and helpers"],
 "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
 "residual_risks": ["Actual Vue components with synthetic accounts and intercepted APIs are development evidence, not a production page login or final acceptance.", "Go, PostgreSQL, full frontend gates, exact SHA deployment and user browser acceptance are tracked by the cumulative release coordinator.", "Production 200% zoom and real account inventory must be checked at final browser acceptance."]
}
-->

## Scope

按固定上游 v2.21.2/dbed7697 移植 OAuth 首次配置、独立并发升级、降智/429 条件动作和指定 API Key 兜底；沿用现有 `/admin/account-quality`，不新增页面或导航。BPS 默认配置、模型计价、2FA 准入和通用告警不属于本工作包。用户正式目标已授权完整移植本工作包。

## Baseline

实现前读取本目录页面、同类账号设置页、ModelWhitelistSelector、共享 Icon、表单按钮、原质量历史与规则编辑组件。基线使用原 AccountQualityView 实际渲染：合成账号、空历史与规则，没有生产读写。基线截图及原型先于新增面板实现生成。

## Prototype

使用固定上游真实 Vue 控件在内存构建中渲染，先生成 PNG 再写可见源代码。保留上游条件、动作、独立开关、选定账号和历史关系，去除本包之外的控制面板。原型完成 360/768/1280/1920px × 中英文 × 浅深色共16组溢出和 pageerror 检查。原型的上游说明中涉及 BPS/其他供应商的文字已在实现中缩至本包实际范围。

## Reuse Decision

复用现有 AppLayout、input/btn、Icon、ModelWhitelistSelector 与现有质量页。面板为原页面内可收起区段；首次打开才加载配置，收起不丢草稿。上游账号选择器用于同页多选备用账号，保留跨页完整加载、搜索与分组过滤。无新路由、私有页面宽度、手写 SVG 或图标体系。

## State Coverage

测试覆盖独立保存开关、配置/历史加载失败重试、保存失败保留输入、缺失分组校验、状态刷新不覆盖草稿、质量规则快照复制、身份切换清除管理员草稿、备用账号分页/过期/影子账号过滤、失败保留已选ID、允许关闭失效策略、日志过滤与继续分页失败重试。hover/active/focus 使用共享控件样式；历史刷新动画遵守 reduced-motion。

## Viewport Coverage

真实组件捕获检查 360、768、1280、1920px × 中英文 × 浅深色，无页面横向溢出。手机上条件、动作和账号选择依次纵向排列；宽屏按上游双列显示。浏览器检查 Space 切换和 Enter 收起/展开及草稿保留。表格历史允许业务区域横向滚动。200% 真实生产整页仍由最终验收补查。

## Evidence

截图由 Chromium 执行真实 Vue 组件生成，浏览器仅加载拦截域下内存构建资源，没有启动开发服务器、连接真实上游或修改生产设置。基线/原型/改后截图使用相同合成账号以便比较。focused 5文件58项与改动控件 ESLint 已通过；完整构建、数据库及后端门禁由累计发布记录维护，未在本记录冒充完成。

## Residual Risk

上述开发组件截图不能证明生产已部署、真实数据库指标正确或用户角色授权已完成最终验收。需在集成精确SHA部署后，核查实际管理员配置持久化/同组多模型、失败冷却/恢复历史、指定备用账号与普通用户拒绝访问，并回归游客/普通用户/管理员三身份。
