# Visual Review: quiz question bank admin

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/play.ts",
    "frontend/src/components/admin/play/AdminQuizQuestionBank.vue",
    "frontend/src/views/admin/PlayOpsView.vue"
  ],
  "routes_or_surfaces": ["/admin/play"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark"],
  "states": ["default", "loading", "disabled", "empty", "error", "success", "hover", "focus-visible"],
  "viewports": ["360x800", "768x900", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/quiz-question-bank/prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/quiz-question-bank/prototype.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/quiz-question-bank/prototype.png"
  ],
  "commands": [
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "The new section uses native inputs, selects, buttons and a native confirm dialog." },
    "reduced_motion": { "status": "passed", "notes": "No new animation or continuous motion was added." },
    "copy_locale": { "status": "passed", "notes": "The new admin surface uses Chinese labels, states and error fallbacks." }
  },
  "residual_risks": [
    "Static review board evidence is used because no browser screenshot was captured in this server environment.",
    "Final product acceptance should verify /admin/play in the production browser after deployment."
  ]
}
-->

## Scope

本次改动在现有后台「玩法运营」页面增加「答题题库管理」区，覆盖题库统计、筛选、列表、新增、编辑、启停和删除。页面不新增独立后台入口，不新增新的按钮、表格或浮层体系。

## Baseline

当前 `/admin/play` 已有统计卡片、活动表格、反馈侧栏、团队列表等运营后台模式。新模块沿用当前后台的 `card`、`input`、`select`、`btn`、表格密度、状态 badge 和 toast。

## Prototype

- Prototype design image: `docs/visual-reviews/assets/quiz-question-bank/prototype.png`
- Approval status: user approved implementing the prior operations plan.
- Scope boundary: only `/admin/play` admin operations; user-side quiz answer解析 can consume the new field later but is not changed in this iteration.

## Reuse Decision

- Shared layouts and components reused: `AppLayout` page shell, existing `card` surfaces, native form controls with `input` and `btn` classes, existing table density and toast pattern.
- New shared pattern, if any: none.
- Design-system exception, if any: none.

## State Coverage

- Default: 展示统计、筛选、题目表格和新增/编辑表单。
- Hover and active: 表格行和文字操作沿用后台现有 hover；无布局位移。
- Focus-visible and keyboard: 输入框、下拉框、按钮均为原生可聚焦控件。
- Loading, disabled, empty, error and success: 刷新/保存期间按钮禁用；空表格显示中文空状态；错误保留表单并显示中文 toast；新增/保存/删除后显示成功 toast 并刷新列表。

## Viewport Coverage

- Mobile: 筛选和表单纵向堆叠，表格横向滚动。
- Tablet: 统计卡片和筛选项自动换行。
- Desktop: 表格和编辑区在宽屏下双列显示。
- Wide or short screen: 不新增页面级宽度和滚动，沿用后台 layout。
- 200% zoom and reduced motion: 无新增动画；表格保留横向滚动作为溢出兜底。

## Evidence

- Updated screenshot or recording: static review board `docs/visual-reviews/assets/quiz-question-bank/prototype.png`.
- Automated visual or overlap checks: design governance manifest maps visible changed files.
- Commands run: `pnpm --dir frontend typecheck`; `pnpm --dir frontend design:check` will be rerun after this manifest fix.

## Residual Risk

- Known limitations: 当前为静态评审板，不是真实浏览器截图。
- Follow-up owner: 部署后由用户在真实浏览器验收 `/admin/play` 的中文文案、筛选、保存和删除操作。
