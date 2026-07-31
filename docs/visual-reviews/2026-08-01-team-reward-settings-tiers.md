# Visual Review: team reward settings tiers

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/play/TeamRewardSettings.vue",
    "frontend/src/components/admin/play/__tests__/TeamRewardSettings.spec.ts"
  ],
  "routes_or_surfaces": ["/admin/settings feature switches: team shared rewards"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "validation error", "loading", "disabled", "success", "hover", "focus-visible"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/play-v2-unification/blindbox-prototype-v2.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/play-v2-unification/blindbox-prototype-v2.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/play-v2-unification/blindbox-prototype-v2.png"
  ],
  "commands": ["pnpm design:check", "pnpm lint:check", "pnpm typecheck", "pnpm test"],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Native inputs retain labels and the add/remove buttons expose accessible names." },
    "reduced_motion": { "status": "passed", "notes": "Only the existing loading spinner is used; add and remove actions do not move surrounding content." },
    "copy_locale": { "status": "passed", "notes": "The component keeps paired Chinese and English text through its existing locale helper." }
  },
  "residual_risks": ["A rendered browser capture is still required before production acceptance."]
}
-->

## Scope

在既有“团队共享奖励”表格中加入新增和删除挡位操作，并在保存前显示阈值、比例、金额和月份的本地校验错误。不会改变结算规则、历史结算或其他系统设置。

## Baseline

用户提供的基线画面显示当前设置页仅能修改既有四档，且比例递减时保存仅收到通用 `internal error`。

## Prototype

复用现有盲盒奖池的紧凑表格、图标操作和按钮层级作为原型，不引入新的页面框架、颜色、圆角或浮层。

## Reuse Decision

保留现有系统设置卡片、表格、`btn`、输入框、Toast、`Icon.vue` 和横向溢出容器。删除动作沿用盲盒奖池的垃圾桶图标按钮，新增操作沿用现有次级按钮。

## State Coverage

- Default: 显示排序后的阈值、比例、奖池上限和开始月份。
- Validation error: 保留输入内容，说明哪一条排序或数值规则不成立，并禁用保存。
- Loading and success: 继续使用已有加载状态和 Toast，不改变按钮尺寸。
- Disabled: 仅剩一档、达到 32 档或当前末档已无法生成更高比例时禁用对应操作。
- Hover and focus-visible: 复用现有 `btn`、输入框和图标按钮状态。

## Viewport Coverage

360px 下表格保留现有横向滚动，新增与删除操作可到达；768px 和 1280px 下复用完整管理工作区宽度。中英文标签和 200% 缩放下操作列不会遮挡数值输入。

## Evidence

- 复用原型：`docs/visual-reviews/assets/play-v2-unification/blindbox-prototype-v2.png`。
- 组件测试覆盖无效比例、增加和删除挡位、有效配置保存。
- 类型检查和设计门禁命令记录在 manifest 中。

## Residual Risk

该记录当前复用了现有静态原型作为交互模式证据；实现后需补充真实浏览器截图以及生产环境管理员验收。
