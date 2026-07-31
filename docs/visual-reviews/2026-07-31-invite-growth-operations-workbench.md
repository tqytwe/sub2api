# Visual Review: invite growth operations workbench

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/play.ts",
    "frontend/src/components/admin/play/AdminInviteGrowthOperations.vue",
    "frontend/src/components/admin/play/__tests__/AdminInviteGrowthOperations.spec.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts"
  ],
  "routes_or_surfaces": ["/admin/play-ops?tab=invite-growth"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "disabled", "empty", "error", "success", "hover", "focus-visible"],
  "viewports": ["360x800", "768x900", "1280x900", "1600x1000"],
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
    "pnpm design:check",
    "pnpm lint:check",
    "pnpm typecheck",
    "pnpm exec vitest run src/components/admin/play/__tests__/AdminInviteGrowthOperations.spec.ts src/views/admin/__tests__/PlayOpsView.spec.ts"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Campaign rows support Enter, tabs use tab roles, and every operation uses native controls." },
    "reduced_motion": { "status": "passed", "notes": "No continuous or layout motion was introduced." },
    "copy_locale": { "status": "passed", "notes": "All new labels, statuses, review roles, ledgers, budget warnings, and debt actions have paired Chinese and English keys." }
  },
  "residual_risks": [
    "The available prototype is a static review board, not a browser capture of the implemented component.",
    "Repository policy forbids a local dev server as production acceptance; final screenshots and browser acceptance remain required on the approved environment.",
    "The backend currently validates review type values but does not expose role-capability metadata for disabling approval actions by operator role."
  ]
}
-->

## Scope

在现有 `/admin/play-ops` 的“邀请增长”TAB 内补齐邀请活动列表、草稿创建与编辑、理论最大负债、状态流转、运营/财务/风控/体验四方审批、参与者/邀请/奖励关联账本，以及 `debt_review` 追缴处理。没有新增路由、顶级入口或设计体系。

## Baseline

基线组件只有四项全局统计、脱敏排行榜和静态领取说明。运营无法在页面创建邀请活动、核对单场规则、完成审批、查看关联明细或处理追缴记录。

## Prototype

- Prototype design image: `docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png`
- Prototype source: `docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.html`
- Approval status: 用户已确认邀请增长、运营管理、APP 数据与八 TAB 信息架构，并要求继续开发完善。
- Scope boundary: 复用现有后台卡片、表格、表单、按钮、`BaseDialog`、`Icon.vue` 和 URL TAB；不新增顶级入口。

## Reuse Decision

- 活动列表、状态徽标、统计数据、筛选和分页复用现有 Play Ops 工作区密度。
- 草稿和追缴使用共享 `BaseDialog`，操作反馈使用现有 Toast。
- 移动端让数据表在明确容器内横向滚动，标题、筛选和关键操作纵向排列。
- 负债提示使用语义 warning 色，不增加渐变、彩色阴影、嵌套卡片或持续动画。

## State Coverage

- Default: 全局统计、活动列表、所选活动摘要、治理操作和关联账本。
- Loading: 汇总、活动、详情和账本分别保持忙碌状态并阻止重复操作。
- Empty: 分别覆盖无活动、未选择活动和筛选无关联记录。
- Error: 主加载错误就地显示并提供重试；操作错误保留输入并使用 Toast。
- Success: 创建、编辑、状态、审批和追缴成功后重新读取服务端版本与账本。
- Disabled: 不可达状态不显示动作；处理中禁用重复提交；追缴依据少于 10 字时禁用决策。
- Hover/focus-visible: 可选活动行支持鼠标与 Enter；原生输入、按钮和 TAB 保留现有焦点样式。

## Viewport Coverage

- 360x800: 标题操作、筛选和治理区域纵向排列；表格只在自己的容器内滚动。
- 768x900: 活动表单时间与阈值自动变为两列，审批块保持可扫描。
- 1280x900 以上: 活动列表与规则摘要并列，账本使用侧栏右侧全部可用宽度。
- 中英文和 200% 缩放仍需在最终批准环境补充真实浏览器证据。

## Evidence

- Pre-implementation prototype: `docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png`.
- Component behavior: `AdminInviteGrowthOperations.spec.ts` covers initial selection, liability calculation, draft creation, status, approval, linked ledgers, and debt resolution.
- Browser captures: pending approved-environment acceptance.

## Residual Risk

- 静态原型不能代替真实页面截图、深浅主题和移动端浏览器验收。
- 后端没有返回当前管理员审批角色能力，前端只能展示四类审批动作，由服务端最终拒绝无权操作。
- 理论最大负债是运营压力测试，不代表后端已按报名人数预留全部预算。
