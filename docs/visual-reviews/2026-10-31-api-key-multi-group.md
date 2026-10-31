# Visual Review: API key multi-group

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/keys.ts",
    "frontend/src/components/account/UpstreamBillingRateCell.vue",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/types/index.ts",
    "frontend/src/views/user/KeysView.vue"
  ],
  "routes_or_surfaces": [
    "/keys create/edit dialog and group table cell",
    "/admin/accounts upstream billing cell"
  ],
  "languages_and_themes": [
    "en-US/light",
    "en-US/dark",
    "zh-CN/light"
  ],
  "states": [
    "default",
    "expanded details",
    "selected checkbox",
    "disabled maximum count",
    "stale historical snapshot",
    "probe loading",
    "API error handled by existing toast"
  ],
  "viewports": [
    "360x800",
    "768x1024",
    "1280x900",
    "1920x1080"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/api-key-multi-group/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/api-key-multi-group/baseline-keys-1280.png",
    "docs/visual-reviews/assets/api-key-multi-group/baseline-billing-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/api-key-multi-group/updated-keys-360.png",
    "docs/visual-reviews/assets/api-key-multi-group/updated-keys-768.png",
    "docs/visual-reviews/assets/api-key-multi-group/updated-keys-1280.png",
    "docs/visual-reviews/assets/api-key-multi-group/updated-keys-dark-1920.png",
    "docs/visual-reviews/assets/api-key-multi-group/updated-billing-360.png",
    "docs/visual-reviews/assets/api-key-multi-group/updated-billing-1280.png",
    "docs/visual-reviews/assets/api-key-multi-group/updated-billing-dark-360.png",
    "docs/visual-reviews/assets/api-key-multi-group/updated-zh-keys-360.png",
    "docs/visual-reviews/assets/api-key-multi-group/updated-zh-billing-360.png"
  ],
  "commands": [
    "vitest run KeysView.capture.spec.ts UpstreamBillingRateCell.capture.spec.ts -t \"capture actual\" (temporary fixture mounts real Vue components and writes their DOM; fixture removed after capture)",
    "tailwindcss -c tailwind.config.js -i fixture.css -o style.css (actual global CSS and existing Select component styles)",
    "node /data/tmp/api-key-multigroup-visual/capture.mjs (Chromium browser CDP screenshot; Emulation.setDeviceMetricsOverride and Page.captureScreenshot)",
    "pnpm design:verify; pnpm lint:check; pnpm typecheck; pnpm exec vitest run; pnpm build"
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
    "Automated component fixture captures use synthetic non-secret data and actual Vue components/CSS. They are not production screenshots or final browser acceptance.",
    "The authenticated full page, Vue Select focus trap and user/admin permissions still require local production browser acceptance after deployment; no development server was started.",
    "Prototype annotates real candidate baseline 04287be20; the baseline is not claimed to be the production UI.",
    "200% browser zoom and authenticated whole-page theme/locale checks remain for production acceptance."
  ]
}
-->

## Scope

本次只扩展现有 `/keys` 分组绑定与 `/admin/accounts` 上游倍率明细。沿用生产分支已有界面；没有新增页面、路由或独立设计原型。

## Baseline

候选提交 `04287be20` 的真实 Vue 组件在 Vitest 中挂载，使用已有非敏感测试数据，输出 DOM 后由 Chromium 载入实际项目 CSS 截图。基线不是线上截图。展开倍率详情挤在固定 24px 高度横排中发生内容遮挡；默认组切换还会丢失原绑定，这由失败测试独立复现。

## Prototype

`assets/api-key-multi-group/prototype-1280.png` 是真实候选组件基线加变更边界注释：保留现有表单、分组选择器与倍率探测按钮；附加组由 Dialog 管理滚动；详情置于倍率横排下方；停用、过期倍率不得混入当前可用范围。用户已授权多分组实现及修复，此记录不声称获得另一次图片确认。

## Reuse Decision

复用 AppLayout、TablePageLayout、BaseDialog、Select、GroupBadge、GroupOptionItem、HelpTooltip、Icon 和既有 checkbox/input/button 语义样式。只在已有字段区域添加原生复选框和 details/summary，无新共享设计模式、视觉例外或页面宽度规则。

## State Coverage

- 默认/选中：真实组件 DOM 保留 input value 和 checked 属性截图；默认组优先、附加组去重。
- 禁用/加载：组件测试检查第 21 个绑定禁选、已选仍可移除，探测中禁用按钮及无重复 probe 事件。
- 空/停用/过期：无可用倍率显示 `-`；停用组独立说明；过期明细仅标最后探测值。
- 成功/失败：真实 Vue 测试验证 create/update payload 和已有 Toast 路径；沿用现有保存失败保留输入行为。
- Hover：不新增位移；沿用按钮和 Select 颜色变化。Chromium CDP 已验证 summary、checkbox 使用 Space 改变展开/选中状态，加 Vue 提交集成测试，不能替代完整页面焦点圈定验收。
- Reduced motion：本次没有新增动画；截图使用 prefers-reduced-motion: reduce。探测动画保持已有 Icon 行为，完整页面既有动画策略不在本次改动范围。

## Viewport Coverage

CDP 设置真实 CSS viewport：360x800、768x1024、1280x900、1920x1080；检查 `documentElement.scrollWidth === innerWidth`，Dialog 边界始终在 viewport 内，details 没有横向溢出。覆盖英语浅/深色和中文浅色。未把 Chromium CLI 最小窗口尺寸造成的裁切误认为产品问题。

## Evidence

所有 PNG 均为 Chromium 对真实 Vue 组件 DOM 的渲染。截图只包含本任务组件，不伪造后台侧栏或线上会话。测试 fixture 与 CDP 临时脚本位于 `/data/tmp/api-key-multigroup-visual`，不进入 frontend 或应用产物。

基线、方案和改后图片见 manifest。行为测试保留在相邻的 KeysView 与 UpstreamBillingRateCell spec 中。design:verify（7 个规范测试与设计增量检查）、完整 ESLint、最终 typecheck、相关 29 个组件测试通过。全量前端 519 个文件通过、3 skipped；4016 tests passed、8 skipped。最终 `pnpm build` 通过（Vite 5m52s，包含 i18n 3/3、vue-tsc -b 和静态产物构建），最终串行链退出码 0。首次并发构建曾被系统以 137 终止、首次全量测试曾以 143 中断；本次串行完整重跑已消除验证缺口。

## Residual Risk

完整生产页面的真实数据、权限、完整焦点圈定和 200% 缩放仍需部署后由用户本地浏览器检查；这些自动化组件截图不作为生产通过证据。范围保持 API key 多分组与倍率声明，不加入正文采集或其他上游功能。
