# TPS 批次共享提示框的调用方宽度修复

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/components/common/HelpTooltip.vue"],
  "routes_or_surfaces": ["/admin/accounts account homepage HelpTooltip", "/usage", "/admin/usage"],
  "languages_and_themes": ["en/light", "zh/dark"],
  "states": ["long account hostname hover", "caller max-w-sm", "viewport cap and restoration", "hover leave and reopen", "existing TPS scroll resize edge keyboard regressions"],
  "viewports": ["1280x900", "360x900", "960x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/v2103-tooltip-width/baseline-accounts-1280.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/v2103-tooltip-width/prototype-accounts-1280.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v2103-tooltip-width/updated-accounts-1280.png",
    "docs/visual-reviews/assets/v2103-tooltip-width/updated-accounts-360.png",
    "docs/visual-reviews/assets/v2103-tooltip-width/updated-accounts-960.png",
    "docs/visual-reviews/assets/v2103-tooltip-width/updated-accounts-1920.png"
  ],
  "commands": [
    "vitest run src/views/admin/__tests__/AccountsView.sparkShadow.spec.ts src/components/common/__tests__/HelpTooltip.spec.ts src/utils/__tests__/usageTps.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts src/views/user/__tests__/UsageView.spec.ts src/views/admin/__tests__/UsageView.spec.ts src/components/account/__tests__/UpstreamBillingRateCell.spec.ts",
    "Playwright Chromium: unchanged AccountsView homepage HelpTooltip template and URL helper extracted into a synthetic fixture, real HelpTooltip and Tailwind, viewport resize and hover",
    "Playwright: rerun the 18 TPS document/nested scroll, resize, edge and keyboard checks against the updated component",
    "pnpm design:check"
  ],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": [
    "Synthetic account-cell and usage-table fixtures do not replace full-page independent review or production acceptance.",
    "360px checks only the shared tooltip viewport cap; mobile product work and Canvas remain deferred.",
    "Await independent review and all CI for the new commit."
  ]
}
-->

## Scope

PR #338 第二轮独立审查发现：共享 HelpTooltip 的内联 max-width 覆盖 AccountsView 账号名称单元格的 `w-max max-w-sm break-all`。本次只修复调用方宽度约束与视口上限的组合；不修改页面、全局样式或 TPS 业务口径。

## Baseline

基线 `a5dca0465a0fe8cd49b32eb8d2a356e45eea6b15`。已阅读 AccountsView 的真实调用、UpstreamBillingRateCell 的自定义宽度调用及共享 HelpTooltip、前端规范。独立审查长域名宽度约 1099.17px；本地合成长域名复现为 1015.78125px，均超过 `max-w-sm` 的 384px。

浏览器夹具从 AccountsView 原样提取账号主页 HelpTooltip 模板和 accountHomepageUrl 函数，使用真实组件、URL 清洗工具及生产 Tailwind CSS。合成账号不含真实凭据；没有启动应用服务。

## Prototype

修改组件前，先捕获基线，再移除旧弹层的内联 max-width 并重新定位，保存 DOM 原型。原型宽度 384px；`baseline-metrics.json` 同时保留修改前与原型尺寸。此原型只确认授权的最小修复边界。

## Reuse Decision

保留现有组件和所有调用方 class。定位前释放上一次视口上限，以调用方实际布局测量宽度；仅在超过视口减去两侧 8px 时临时收窄，再测量高度并沿用已有定位逻辑。视口变大后恢复调用方限制。没有增加全局样式或替换调用方 max-width。

## State Coverage

AccountsView 新增真实页面挂载测试，使用实际账号名称单元格与 HelpTooltip。RED 阶段先证明旧代码的 `calc(100vw - 16px)` 覆盖调用方约束。最终页面测试直接断言正常视口不写入覆盖值、窄视口收窄及恢复；复用已有账号页 class 合同断言，不注入或豁免全局样式。JSDOM 尺寸是明确标注的布局模型，384px 实际 CSS 结果由浏览器截图与测量验证。7 个定向测试文件共 99 项通过。

浏览器验证 1280/960/1920 下保持 384px，360 下收窄为 344px，hover 离开关闭并可再次打开。既有 TPS 用户/管理员组件的 18 项滚动、嵌套滚动、resize、四边、Enter/Escape 和焦点检查继续通过。数据加载、空、失败、禁用、保存成功状态不受宽度修复影响，复用前批页面测试；本次不新增业务交互状态。

## Viewport Coverage

账号提示框 1280×900 浅色、1920×900 深色及 resize 到 360×900/960×900。360 只验证弹层机械边界，不开展移动端页面整改。TPS 回归继续覆盖中英文、浅深色、1280/1920 及 resize960；启用 reduced-motion。

## Evidence

本目录记录的基线、原型和修改后 PNG 均为 Chromium 实际渲染；`assets/v2103-tooltip-width/browser-checks.json` 保存账号宽度及命中检查，`tps-regression-checks.json` 保存旧 18 项定位回归结果。原型及修改后图片分别记录，不以 DOM 原型冒充组件实现。

## Residual Risk

等待新 SHA 的完整页独立复审及全部 CI。合成夹具不是生产验收；生产部署、用户本机游客/普通用户/管理员验收仍由主线程协调。本次不代表整体核心迁移完成。
