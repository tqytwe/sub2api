# Visual Review: openai-request-timezone

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/OpenAIRequestTimezoneField.vue",
    "frontend/src/api/admin/accounts.ts",
    "frontend/src/api/admin/settings.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts"
  ],
  "routes_or_surfaces": ["/admin/settings", "/admin/accounts (create/edit modal)"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "hover", "focus-visible", "loading", "error"],
  "viewports": ["360x800", "768x900", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/openai-request-timezone/prototype-settings-zh-light-1280.png",
    "docs/visual-reviews/assets/openai-request-timezone/prototype-settings-en-dark-1280.png",
    "docs/visual-reviews/assets/openai-request-timezone/prototype-account-field-zh-light-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/openai-request-timezone/before-settings-zh-light-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/openai-request-timezone/after-settings-zh-light-1280.png"
  ],
  "commands": [
    "python static PNG artifact generation for request timezone settings and account field review boards",
    "pnpm vitest run src/components/account/__tests__/OpenAIRequestTimezoneField.spec.ts",
    "npx vue-tsc --noEmit -p tsconfig.json"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Only shared Toggle and Select components are added; they carry the design-system focus and keyboard behavior unchanged."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No animation, transition, or motion behavior was added or changed."
    }
  },
  "residual_risks": [
    "Static review boards do not replace the final administrator browser screenshot or production acceptance by the user after deployment.",
    "The timezone catalog endpoint must be reachable for the Select to populate; failure state is covered by the load-failed hint line."
  ]
}
-->

## Scope

- Routes: `/admin/settings`（Codex 设置卡新增一个开关）；`/admin/accounts` 创建/编辑 Modal（仅 OpenAI 平台显示时区字段）
- Roles: 管理员
- Languages and themes: 中文/英文 × 浅色/深色（全部复用既有 dark: 变体与 i18n 命名空间）

## Baseline

- Current behavior: 无请求时区设置；Codex 设置卡现有开关与表单不变
- Baseline screenshot or recording: 无（本机无浏览器；本改动只在既有卡片顶部追加一个复用行，不改变既有行布局）
- Inconsistencies observed: 无

## Prototype

- Prototype design image: `assets/openai-request-timezone/prototype-*.png`（静态审查板，非浏览器截图）
- Approval status: 端口自适应移植上游 ranxi2001/sub2api v2.10.5 已发布界面；布局模式与上游一致
- Scope boundary: 仅新增「请求时区绑定」开关行 + OpenAI 账号 Modal 时区 Select 字段 + i18n + API 类型；不重排既有控件

## Reuse Decision

- Shared layouts and components reused: `components/common/Select.vue`（含 searchable/loading）、SettingsView 既有 `Toggle` 行模式（label+desc 左、Toggle 右，与相邻 codex 开关同一模板结构）、既有 `input-label`/`input-hint` 类
- New shared pattern, if any: 无
- Design-system exception, if any: 无；未新增手写 SVG、未新增页面宽度/居中规则

## State Coverage

- Default: 开关默认关闭（form.openai_request_timezone_enabled=false）；时区字段默认 Asia/Singapore
- Hover and active: 复用 Toggle/Select 既有状态
- Focus-visible and keyboard: 复用 Toggle/Select 既有状态
- Loading, disabled, empty, error and success: 时区目录加载中显示 Select loading；加载失败显示红色提示行（requestTimezoneLoadFailed）；目录加载后非法既有值自动回落默认

## Viewport Coverage

- Mobile: 开关行为 flex 行内 min-w-0 截断，与相邻行同模式；Select 为共享组件响应式行为
- Tablet: 同上
- Desktop: 1280 原型见 assets
- Wide or short screen: 卡片高度自然增长一行，无溢出风险
- 200% zoom and reduced motion: 无新增动画；缩放行为与相邻开关行一致

## Evidence

- Updated screenshot or recording: 无（本机无浏览器，见 residual risk）
- Automated visual or overlap checks: `pnpm design:check`（PR CI）；`OpenAIRequestTimezoneField.spec.ts`、`SettingsView.spec.ts` 单元测试
- Commands run: `pnpm vitest run`（frontend tests）、`pnpm typecheck`

## Residual Risk

- Known limitations: 本机无浏览器，基线/改后截图为静态审查板；真实浏览器验收（浅色/深色 × 中英文 × 管理员身份）必须在生产由用户完成，完成前不宣称视觉验收通过
- Follow-up owner: 用户生产浏览器验收
