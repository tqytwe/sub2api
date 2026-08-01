# Visual Review: invite growth step-up guard

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/play/AdminInviteGrowthOperations.vue"
  ],
  "routes_or_surfaces": ["/admin/play-ops?tab=invite-growth"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "disabled", "empty", "error", "success", "focus-visible"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"
  ],
  "commands": [
    "pnpm install --frozen-lockfile",
    "pnpm typecheck",
    "pnpm exec eslint src/components/admin/play/AdminInviteGrowthOperations.vue",
    "./scripts/check-fork-integrity.sh"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "The new save, status, review and debt actions remain native buttons and are gated by the existing TOTP step-up dialog." },
    "reduced_motion": { "status": "passed", "notes": "The change adds no animation or layout motion." },
    "copy_locale": { "status": "passed", "notes": "The new error handling reuses existing zh/en step-up locale keys; no new locale strings were introduced." }
  },
  "residual_risks": [
    "This is a static review board record for the existing workbench layout, not a fresh browser capture of the gated dialog after the latest code change.",
    "Final production acceptance still needs the merged PR to pass GitHub checks and the protected route to be verified live."
  ]
}
-->

## Scope

在现有 `/admin/play-ops` 的“邀请增长”TAB 内，为新增/编辑活动、状态流转、四方复核和坏账处理补上 TOTP step-up 门控。没有新增路由、顶级入口或页面布局。

## Baseline

基线页面已经有邀请增长列表、草稿弹窗、审批队列和关联账本，但敏感提交动作没有统一走 step-up，管理员触发后会直接撞到后端最近二次验证门控。

## Prototype

- Prototype design image: `docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png`
- Prototype source: `docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.html`
- Approval status: 延续已确认的玩法运营工作台结构，仅补敏感操作门控，不改页面骨架。
- Scope boundary: 复用现有后台卡片、对话框、按钮、表格和 `TotpStepUpDialog`，不改设计体系。

## Reuse Decision

- 复用现有 `BaseDialog`、`TotpStepUpDialog`、`useStepUp`、按钮和表单控制。
- 不新增视觉层级，不引入渐变、悬浮装饰或额外布局容器。
- 失败文案沿用已有中英文 step-up locale key。

## State Coverage

- Default: 邀请活动编辑、状态变更、审批和坏账按钮保持原位，提交前自动触发二次验证。
- Loading: 既有保存和动作 loading 状态继续禁用重复提交。
- Disabled: 无变更或不足 10 字的坏账说明仍保持禁用。
- Error: step-up 未启用或管理员 API key 禁用时显示已有错误文案。
- Success: 二次验证通过后按原有流程提交并刷新数据。
- Focus-visible: 继续使用原生按钮和对话框焦点样式。

## Viewport Coverage

- 360x800: step-up 弹窗不改变主工作台的单列阅读顺序。
- 768x900: 表单、动作区和账本仍保持原有两列布局。
- 1280x900: 工作台继续占满右侧可用宽度，没有新增私有页面宽度。

## Evidence

- Static prototype remains the approved workbench board: `docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png`.
- Verified locally: `pnpm typecheck`, `pnpm exec eslint src/components/admin/play/AdminInviteGrowthOperations.vue`.
- Full fork integrity rerun pending after adding this visual review record.

## Residual Risk

静态审查板不能替代真实浏览器里的 step-up 弹窗截图。最终上线前仍需在合并后的 PR 上确认 GitHub checks 和生产受保护接口。
