# 套餐编辑配置保留

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/orders/PlanEditDialog.vue"],
  "routes_or_surfaces": ["/admin/orders/plans", "PlanEditDialog"],
  "languages_and_themes": ["en/light", "en/dark"],
  "states": ["existing populated plan", "edit", "cancel", "reopen", "save", "keyboard Tab", "reduced motion"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/plan-edit-preserve/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/plan-edit-preserve/before-1280.png", "docs/visual-reviews/assets/plan-edit-preserve/before-360.png", "docs/visual-reviews/assets/plan-edit-preserve/before-768.png", "docs/visual-reviews/assets/plan-edit-preserve/before-1920.png", "docs/visual-reviews/assets/plan-edit-preserve/before-dark.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/plan-edit-preserve/after-1280.png", "docs/visual-reviews/assets/plan-edit-preserve/after-360.png", "docs/visual-reviews/assets/plan-edit-preserve/after-768.png", "docs/visual-reviews/assets/plan-edit-preserve/after-1920.png", "docs/visual-reviews/assets/plan-edit-preserve/after-dark.png"],
  "commands": ["Playwright Chromium: PLAN_BROWSER_CONTRACT=1 go test ./internal/planacceptance -run TestAdminPlan -v -count=1 (dedicated loopback PostgreSQL + actual migrations)", "pnpm exec vitest run src/views/admin/orders/__tests__/PlanEditDialog.spec.ts src/views/admin/orders/__tests__/AdminPaymentPlansView.spec.ts"],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["This isolated component browser uses the actual view/dialog/API client and HTTP handlers; the unrelated navigation shell is replaced with PageFrame. Screenshots are technical evidence, not production acceptance.", "No template, CSS, icon or layout changes. Comprehensive accessibility, zh language and 200% zoom checks are outside this data repair; production browser acceptance remains with the main thread."]
}
-->

## Scope

用户已授权独立修复套餐编辑数据丢失，要求保留现有前端风格；范围仅为数据回填和保存载荷，未改模板、样式、控件或金额计算。基线由 `a7e1c0033` 普通 fast-forward merge 到 `4080e2ac7`；这两个版本的套餐目标代码完全相同。

## Baseline

实现前阅读 AdminPaymentPlansView、PlanEditDialog、同类 PaymentProviderDialog、相邻 PlanStorefrontConfigPanel 及共享 BaseDialog/Select/PageFrame。浏览器运行真实套餐 GET → 列表 → 编辑弹窗；遗漏字段显示为空/默认值，保存仅改描述也会清空 PostgreSQL 配置。见 [RED 记录](../archive/completed-tasks/2026-10-plan-edit-preserve/red-postgres.log)。

## Prototype

原型 [prototype-1280.png](assets/plan-edit-preserve/prototype-1280.png) 在改实现前从现有弹窗截取；本任务明确沿用这一布局与交互控件，只修复数据，不提出新的视觉模式。用户“保留现有前端风格”的授权确定了边界。

## Reuse Decision

原有 BaseDialog、Select、ImageUpload、GroupBadge、Icon、DataTable 和按钮全部复用；生产模板零改动。测试页面仅剥离无关的全局导航，以实际 PageFrame 包裹实际管理页面；套餐网络响应没有 mock。

## State Coverage

- 已验证非默认字段回填；修改描述；取消（0 个 PUT）；重开恢复数据库值；保存（只发送描述）；再次 GET 及数据库逐字段对账。
- 组件测试覆盖缺字段旧响应、空字符串、false、0、额度明确清除、重新打开、非法价格不写；HTTP 契约覆盖其他非法字段原子拒绝及游客 401/普通用户 403。
- 补充真实 PostgreSQL 浏览器用例覆盖纯空白产品名/详情/徽标明确清空，PUT 仅三字段、GET/DB 对账；19 项弹窗组件测试包含纯空白输入变化。既有列表已过滤为空的字符串 features 在空→空时继续保留，未宣称该路径的页面清空验证。模板/样式未改，沿用本记录截图。
- 键盘 Tab、浅/深色、reduced-motion 下截图；沿用控件的 hover/active/focus 样式。loading/disabled 沿用现有 saving 状态，未新增视觉行为；空列表及错误视觉不属于本次模板变更。

## Viewport Coverage

360、768、1280、1920 × 900 均保存了真实 Chromium 前后截图；弹窗内容使用已有 modal-body 滚动。英文浅色与英文深色/reduced-motion 已采集。未宣称中文、200% 或生产浏览器验收。

## Evidence

自动化入口：[browser runner](../../frontend/e2e/plan-edit/run.mjs)、[Go HTTP/数据库桥接测试](../../backend/internal/planacceptance/plan_browser_test.go)。PostgreSQL 每个用例新建随机命名测试数据库并执行全部仓库迁移，含 273/274；测试结束删除数据库。正常用户权限来自真实 JWT + DB 用户 + AdminAuthMiddleware。

## Residual Risk

这些是隔离技术验证；主线程负责合并、部署和用户本地电脑的游客/普通用户/管理员最终验收。未写入生产套餐、未调用支付或购买，未部署。
