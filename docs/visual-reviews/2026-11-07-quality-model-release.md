# Visual Review: quality model isolation and release

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/AccountQualityView.vue",
    "frontend/src/components/admin/account/QualityIsolationGroups.vue",
    "frontend/src/components/admin/account/ScheduledTestsPanel.vue",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts"
  ],
  "routes_or_surfaces": [
    "/admin/account-quality rule, preset, template and ownership preview",
    "/admin/accounts existing scheduled-test panel central management link"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en/light",
    "en/dark"
  ],
  "states": [
    "isolation scope retained in edit/preset/template",
    "group loading/error/empty/missing selection",
    "explicit reason and confirmation",
    "disabled unsafe marker and last model",
    "conflict refresh without retry",
    "success scope/preview readback",
    "native keyboard/focus and reduced motion"
  ],
  "viewports": [
    "360x1000",
    "768x1000",
    "1280x1000",
    "1920x1000"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/quality-model-release/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/quality-multiprobe/updated-zh-form-1280.png",
    "docs/visual-reviews/assets/quality-group-rules-20261031/baseline-scheduled-tests-production.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/quality-model-release/ownership-360-light.png",
    "docs/visual-reviews/assets/quality-model-release/ownership-360-dark.png",
    "docs/visual-reviews/assets/quality-model-release/ownership-1280-light.png",
    "docs/visual-reviews/assets/quality-model-release/ownership-1280-dark.png",
    "docs/visual-reviews/assets/quality-model-release/groups-360-dark.png",
    "docs/visual-reviews/assets/quality-model-release/template-360-dark.png",
    "docs/visual-reviews/assets/quality-model-release/conflict-360-dark.png",
    "docs/visual-reviews/assets/quality-model-release/released-360-dark.png"
  ],
  "commands": [
    "QUALITY_PLAYWRIGHT_MODULE=/path/to/playwright-core/index.mjs node docs/archive/completed-tasks/2026-11-07-quality-model-release/verify-render.mjs (offline Playwright real Vue component)",
    "pnpm exec vitest run src/views/admin/__tests__/AccountQualityView.spec.ts src/components/admin/account/__tests__/ScheduledTestsPanel.qualityOwnership.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts",
    "make test; make test-backend-unit; make build; ./scripts/check-fork-integrity.sh"
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
    "Offline real-component screenshots use synthetic API responses and a layout stub; they do not replace authenticated production or user local browser final acceptance.",
    "The existing scheduled-test panel link is covered by Vue tests; full modal geometry and three production identities remain local browser acceptance.",
    "200% browser zoom and complete production shell focus order require local browser acceptance."
  ]
}
-->

## Scope

沿用质量运维现有入口，补隔离动作与明确分组选择；现有归属预览逐模型展示影响、原因、禁止条件与确认。账号定时测试面板的隔离计划链接到同一管理页，不增加导航或独立管理面。

## Baseline

实现前查看真实组件历史截图（规则表单、账号定时测试弹窗）、目标页面代码、相邻 PrioritySchedulingView 和现有 Toggle、Select、GroupSelector。GroupSelector 的 simple-mode watcher 会主动删选中值，不适合归属配置必须无损回读的合同，因此不复用其变更逻辑。

## Prototype

修改可见代码前保存并展示 prototype-1280.png。它是明确标识的静态审查板，规定原页面新增隔离动作/分组和逐模型释放，没有更换页面结构。用户已授权这一功能完整实现，不增加审批步骤。

## Reuse Decision

复用 AppLayout、现有表单网格、input、btn、Toggle、Icon、原生 checkbox/select 与归属预览区域。QualityIsolationGroups 仅封装两处相同分组选择，保留不可用分组 ID；没有新颜色、浮层、图标或页面宽度。账号面板复用标准链接跳转统一入口。

## State Coverage

默认/编辑/预设/模板均保留 remove_group_ids；读取失败有明确错误并阻止隔离写入，不静默清空范围；缺失分组继续显示旧选择。释放需选原因与确认，每次只改一个模型；所有写入中禁用操作。后端 conflict 后重新预览但保留错误、清空旧选择，不能自动重放请求。操作成功重新读取计划和预览后才提示成功，并同步移除当前编辑草稿中的目标，保留其他草稿。规则读取失败、预览失败或旧目标仍存在时，提示“已提交但未确认”并保留草稿，仍不可释放的其他模型保持原样。

hover/active/focus-visible 复用 btn/input 的既有状态；真实 Chromium 验证无原因时按钮不可点、阻止项不可点、Tab 跳过 disabled 按钮、冲突后重新选原因以及成功回读。成功/错误复用现有 status/alert，未新增动画，reduced-motion 媒体匹配通过。游客和普通用户没有新增操作入口，后端继续走管理员路由组。

## Viewport Coverage

离线编译真实 Vue 页面和生产 Tailwind CSS，16 个宽度×主题×语言场景（360/768/1280/1920，浅深，中英）；检查页面无水平溢出、禁止项和键盘顺序，保存窄屏/桌面归属、分组/模板、冲突及释放后截图。纯开发夹具不调用生产/上游，不启动应用服务。

## Evidence

可复跑脚本位于 docs/archive/completed-tasks/2026-11-07-quality-model-release/verify-render.mjs；geometry.json 保存16场景几何和交互检查。组件行为测试与语言完整性39项通过，包括编辑草稿同步与三种回读异常；真实PG/HTTP证据和完整门禁以本任务交付记录为准。

## Residual Risk

这些截图是实际组件开发证据，不是用户本机或已登录生产证据。部署后还需核对精确 SHA、健康与关键API，并如实保留三身份本机、完整弹窗/整页及200%缩放验收缺口。
