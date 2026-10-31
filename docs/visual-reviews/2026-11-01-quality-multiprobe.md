# Visual Review: quality multiple probes

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/AccountQualityView.vue",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/i18n/locales/en/admin/resources.ts"
  ],
  "routes_or_surfaces": [
    "/admin/account-quality existing rule form, group template form and probe history"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "existing populated edit",
    "additional model add/remove",
    "legacy zero count normalization",
    "invalid round count and corrected save",
    "preset/template retention",
    "history success/inconclusive/skipped",
    "native keyboard focus and activation"
  ],
  "viewports": [
    "360x950",
    "768x950",
    "1280x950",
    "1920x950"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/quality-multiprobe/baseline-zh-scope-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/quality-multiprobe/baseline-zh-form-1280.png",
    "docs/visual-reviews/assets/quality-multiprobe/baseline-zh-template-1280.png",
    "docs/visual-reviews/assets/quality-multiprobe/baseline-zh-history-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/quality-multiprobe/updated-en-form-dark-1920.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-en-form-dark-360.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-en-history-dark-1280.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-en-template-1280.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-en-template-dark-360.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-zh-form-1280.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-zh-form-360.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-zh-form-768.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-zh-history-360.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-zh-history-dark-768.png",
    "docs/visual-reviews/assets/quality-multiprobe/updated-zh-template-360.png"
  ],
  "commands": [
    "QUALITY_CAPTURE_STAGE=updated QUALITY_CAPTURE_LOCALE=zh/en pnpm exec vitest run src/views/admin/__tests__/AccountQualityView.capture.spec.ts (temporary real Vue DOM export, fixture removed after capture)",
    "pnpm exec tailwindcss -c tailwind.config.js -i /data/tmp/quality-multiprobe-visual/fixture.css -o /data/tmp/quality-multiprobe-visual/style.css",
    "node /data/tmp/quality-multiprobe-visual/capture.mjs updated (Chromium browser CDP screenshot; actual CSS viewport and geometry)",
    "pnpm exec vitest run src/views/admin/__tests__/AccountQualityView.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts",
    "pnpm design:check; pnpm exec eslint src/views/admin/AccountQualityView.vue src/views/admin/__tests__/AccountQualityView.spec.ts src/i18n/locales/zh/admin/resources.ts src/i18n/locales/en/admin/resources.ts; pnpm typecheck"
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
    "Component fixture captures use synthetic non-secret data and actual Vue DOM/CSS. They are development evidence, not authenticated production screenshots or final local browser acceptance.",
    "CDP native keyboard activation is checked against exported DOM; actual Vue model add/remove and API payload behavior are covered by component tests separately.",
    "Authenticated whole-page layout, 200% browser zoom, complete focus order and real administrator actions require local production browser acceptance after deployment."
  ]
}
-->

## Scope

沿用 `/admin/account-quality` 现有质量规则、分组模板、预设和历史区域，只补已存在配置字段的多模型/每模型探测次数，以及结果模型、轮次和跳过计数。没有新页面或独立产品原型。

## Baseline

从实际生产源码 #396 `73f9ba38b4d61ad5a0e0ac05ab2004d4c9775444` 的 `AccountQualityView.vue` 挂载真实 Vue DOM，使用非敏感测试数据及项目 CSS。`assets/quality-multiprobe/baseline-zh-{form,template,history}-1280.png` 为浏览器截图，非线上登录截图。现有表单只显示单个模型，保存时丢弃已配置的 model_ids/parallel_count；历史没有区分每个模型和轮次。

## Prototype

实现前已保存 `assets/quality-multiprobe/baseline-zh-scope-1280.png`：在真实现有组件基线上方标注改动边界，保留原布局和已有动作。它是开发审查范围图，不是另一套虚构产品界面。用户已授权按现有生产界面补齐本批能力，无需另行扩大范围。

## Reuse Decision

复用 AccountQualityView、现有 `.input`、`.btn`、Icon、Toggle、表单网格与历史列表；不新增页面框架、浮层、图标或自定义颜色。

## State Coverage

- 默认/编辑：规则、预设和分组模板保留多模型及每模型次数；显式model_ids集合优先于过时legacy主模型，编辑回读以显式首项为主模型，列表也不擅自追加旧主模型（新增3项先失败再通过的回归）；旧值 0 回读为 1；切换预设与保存后重置不串值。
- 增删与校验：附加模型复用原生 input 和 Icon 按钮，重复模型只计算一次。每模型 1–32 次、最多 50 模型、单轮最多 100 个 probe；超过 100 提示错误并保留输入，改正可保存。组件测试验证 payload，不静默截断。
- 历史：成功与 inconclusive 使用已有状态文字，补模型/轮次标识与整轮 skipped 计数；长模型名在窄屏换行，操作仍可到达。
- Hover/active/focus-visible：复用现有 btn/input 状态，不增加动画或位移。Chromium CDP 验证 Tab 从附加模型输入框到有名称的移除按钮、Enter 触发一次原生 click；Vue 中实际移除另由组件测试验证。
- Loading/disabled：沿用现有保存/模板保存 busy 状态，新增增删模型按钮在对应保存中禁用；模型 50 个时禁止继续新增。未新增异步读取或浮层。
- Empty/error/success：沿用初始单模型、历史空/错重试与原有 alert/status；回归覆盖保存成功、校验错误保留值及修正后重试。完整页面焦点路径和触屏仍留最终生产验收。

## Viewport Coverage

Chromium CDP 使用真实 CSS 视口 360、768、1280、1920 x 950；共 11 张修改后截图，覆盖规则表单、分组模板、结果历史，包含中英文与浅深主题。截图含接近 100 字符模型名，逐项测量 input/button/textarea/select 边界，11 例均无控件越出视口、无页面横向溢出；人工查看中文手机表单/模板、英文手机深色表单/模板和英文桌面深色历史。reduced-motion 媒体匹配通过。本批无新增动画，未把组件截图当作整页或 200% 缩放通过证据。

## Evidence

临时 Vitest 挂载真实 Vue 组件、Chromium CDP 渲染真实组件 DOM和项目CSS；fixture和脚本保存在 `/data/tmp/quality-multiprobe-visual`，不作为应用产物提交。截图中的测试账号/模型/轮次均为 synthetic 数据。临时 `AccountQualityView.capture.spec.ts` 已移出 frontend；真正行为回归保留在相邻 `AccountQualityView.spec.ts`。

定向回归：AccountQualityView 21/21、i18n 完整性 3/3，通过；定向 ESLint 通过。显式集合边界修复后再次运行21项组件/3项i18n、定向ESLint、typecheck和设计检查，均退出0（日志`/data/tmp/quality-explicit-models-*`）。设计门禁 `pnpm design:check`（基线 #396）与最终 `vue-tsc --noEmit` 均退出 0；`git diff --check` 通过。后端及全量测试/构建由主集成任务统一执行，避免重复启动重资源任务。

## Residual Risk

这些组件截图不证明生产管理员真实数据、登录页面、权限或完整焦点路径验收。最终生产管理员本机浏览器验收另行记录。
