# 战队到账证明前十条审查

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/public/AgentTeamView.vue"
  ],
  "routes_or_surfaces": [
    "/agent-team 历史结算区的奖励到账证明"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "en-US/light"
  ],
  "states": [
    "历史区已加载并有到账证明",
    "历史区空态",
    "历史区加载中",
    "移动端金额与时间换行"
  ],
  "viewports": [
    "390x844",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/team-reward-showcase-top10/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/team-reward-showcase-top10/baseline-390.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/team-reward-showcase-top10/updated-1280.png"
  ],
  "commands": [
    "pnpm exec vitest run src/views/public/__tests__/AgentTeamView.competitive.spec.ts",
    "pnpm design:check",
    "pnpm typecheck"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "到账证明是只读内容，本次没有新增键盘交互。"
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "本次仅增加金额文本和稳定网格列，不引入动画。"
    }
  },
  "residual_risks": [
    "静态审查板不是生产浏览器截图，发布后仍需在真实历史数据下核对移动端和英文环境。"
  ]
}
-->

## Scope

- 路由：`/agent-team` 的历史结算区域。
- 身份：游客、未入队用户和成员都可查看公开到账证明。
- 范围：只显示已有的到账金额，历史前十战队的排名与金额不变。

## Baseline

原到账证明行展示脱敏昵称、月份、战队名和到账时间，但遗漏接口已返回的金额字段。历史区数据较多时，五十条证明也会使该区域过长。

## Prototype

原型审查板对比同一条到账证明的修改前后：保留头像、脱敏身份、月份、战队和到账时间，在时间前增加金额列。该图片明确标识为静态审查板。

## Reuse Decision

复用现有 `team-proof-row`、`PlayUserAvatar`、金额格式化函数和历史结算加载流程。没有新增组件、图标、交互、颜色或卡片模式；仅扩展现有网格列，移动端保持时间独占下一行。

## State Coverage

- 默认：有到账证明时显示金额。
- 空态、加载和失败：沿用现有历史区文案和状态，不改行为。
- 键盘：只读内容，无新增焦点或操作。
- 成功：金额与历史战队已发放金额可以同时扫描。

## Viewport Coverage

- 390x844：头像、身份和金额同一行，时间换到下一行。
- 1280x820：头像、身份、金额和时间保持四列。
- 768px 及更宽：沿用桌面网格。
- 无新增动画，reduced-motion 无额外影响。

## Evidence

三个 PNG 均由本地 `review-board.html` 实际浏览器渲染生成。定向 Vitest 覆盖历史前十金额与到账金额同时出现；后端处理器测试覆盖公开证明上限为十。

## Residual Risk

静态审查板不替代线上历史数据验收。发布后应由用户在真实 2026-07 历史赛季下确认九支战队及其金额仍显示，到账证明不超过十条。
