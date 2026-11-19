# 定向公告视觉检查

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/AnnouncementsView.vue",
    "frontend/src/components/admin/announcements/AnnouncementTargetingEditor.vue",
    "frontend/src/components/admin/announcements/AnnouncementUserPicker.vue",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts"
  ],
  "routes_or_surfaces": ["/admin/announcements 的创建和编辑弹窗"],
  "languages_and_themes": ["中文/浅色", "英文/深色", "英文/浅色", "中文/深色"],
  "states": ["初始未选择", "搜索为空", "搜索失败后重试", "键盘选择", "删除收件人", "阻止提交空受众", "保存后打开并恢复收件人"],
  "viewports": ["360x800", "768x900", "1280x900", "1920x1080"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-19-targeted-announcements/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-19-targeted-announcements/baseline-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-19-targeted-announcements/updated-1280.png", "docs/visual-reviews/assets/2026-11-19-targeted-announcements/updated-360.png"],
  "commands": ["pnpm -C frontend build", "Playwright Chromium 使用生产构建资源和模拟公告、用户接口检查四种视口"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Tab 进入搜索结果，Enter 选择收件人且不会提交表单，Escape 关闭公告弹窗。"},
    "reduced_motion": {"status": "passed", "reason": "四种视口均在 reducedMotion=reduce 下运行。"}
  },
  "residual_risks": ["服务端浏览器使用模拟账号和接口数据。管理员和普通用户的线上受众隔离仍需在生产环境验收。"]
}
-->

## Scope

在原有公告弹窗选择定向收件人。

## Baseline

当前弹窗支持广播及按现有条件投放。基线图为原版本实际浏览器画面。

## Prototype

原型在现有弹窗中加上指定用户模式，并沿用原布局。

## Reuse Decision

复用共享弹窗、按钮、输入框及语义颜色。组合条件中也可按邮箱或用户名搜索。

## State Coverage

覆盖空选择、搜索空结果、搜索失败、键盘选择、删除、保存及读回。搜索中显示提示，选满后禁用输入。

## Viewport Coverage

在 360、768、1280 和 1920 像素宽度下完成交互检查，收件人控件无横向溢出。

## Evidence

修改后截图取自前端生产构建。模拟接口使用虚构账号，保存后再次打开可读回所选收件人。

## Residual Risk

线上仍需管理员及普通用户验收受众隔离。
