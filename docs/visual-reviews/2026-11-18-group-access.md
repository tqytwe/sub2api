# 分组访问策略视觉审查

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/components/admin/group/GroupUserDeniedModelsModal.vue",
    "frontend/src/components/admin/group/ModelTagInput.vue",
    "frontend/src/api/admin/groups.ts",
    "frontend/src/types/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts"
  ],
  "routes_or_surfaces": [
    "/admin/groups",
    "分组新建和编辑",
    "用户模型限制弹窗"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark"
  ],
  "states": [
    "default",
    "hover",
    "active",
    "focus-visible",
    "loading",
    "disabled",
    "empty",
    "error",
    "success"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-group-access/prototype-form-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-group-access/baseline-list-1280.png",
    "docs/visual-reviews/assets/2026-11-18-group-access/baseline-form-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-group-access/updated-form-1280.png",
    "docs/visual-reviews/assets/2026-11-18-group-access/updated-model-limits-360.png",
    "docs/visual-reviews/assets/2026-11-18-group-access/updated-model-limits-768.png",
    "docs/visual-reviews/assets/2026-11-18-group-access/updated-model-limits-1280.png",
    "docs/visual-reviews/assets/2026-11-18-group-access/updated-model-limits-1920.png"
  ],
  "commands": [
    "Playwright browser screenshot of built application with fixture responses",
    "pnpm design:check",
    "pnpm lint:check",
    "pnpm typecheck"
  ],
  "checks": { "keyboard": { "status": "passed" }, "reduced_motion": { "status": "passed" } },
  "residual_risks": [
    "生产三身份验收待部署后在本机浏览器完成。"
  ]
}
-->

## Scope

现有分组页的创建、编辑和用户模型限制弹窗。现有列表和导航继续承载入口。

## Baseline

使用生产分支构建，在浏览器中打开分组列表和创建表单，接口返回测试数据。

## Prototype

在基线表单中预览三个访问策略开关，用户模型限制入口与分组专属倍率和 RPM 并列。

## Reuse Decision

沿用 AppLayout、DataTable、BaseDialog、Toggle、Pagination 和 Icon。

## State Coverage

已检查键盘切换开关、保存和回填、加载失败后刷新，以及保存失败后保留清空草稿并重试。

## Viewport Coverage

在 360、768、1280、1920 像素宽度下检查模型限制弹窗，覆盖浅色、深色。截图中的语言为中文。

## Evidence

上方记录包含当前生产分支基线、原型和修改后构建的真实浏览器截图。截图使用测试数据。

## Residual Risk

生产三身份验收在部署后执行。
