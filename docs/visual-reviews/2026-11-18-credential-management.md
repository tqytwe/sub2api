# 凭证运营、SIWC 与 2FA 管理

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/CredentialEncryptionSetup.vue",
    "frontend/src/components/account/OpenAITOTPDialog.vue",
    "frontend/src/components/account/OpenAISessionExportDialog.vue",
    "frontend/src/components/account/SIWCAccountSettings.vue",
    "frontend/src/views/admin/ops/TokenGuardV2View.vue",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/zh/tokenGuardV2.ts",
    "frontend/src/i18n/locales/en/tokenGuardV2.ts",
    "frontend/src/i18n/locales/zh/siwc.ts",
    "frontend/src/i18n/locales/en/siwc.ts"
  ],
  "routes_or_surfaces": [
    "/admin/token-guard-v2 credential operations",
    "/admin/accounts existing edit dialog"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en/light",
    "en/dark"
  ],
  "states": [
    "default",
    "hover",
    "focus-visible",
    "loading",
    "disabled",
    "empty",
    "error",
    "success",
    "uncertain TOTP recovery",
    "SIWC bound but protocol disabled",
    "browser authorization",
    "cancelled polling"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-credential-management/baseline-360-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/baseline-1280-light.png"
  ],
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-credential-management/prototype-360-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/prototype-1280-light.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-en-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-en-1280-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-en-1920-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-en-1920-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-en-360-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-en-360-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-en-768-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-en-768-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-siwc-en-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-siwc-en-1280-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-siwc-en-360-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-siwc-en-360-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-siwc-zh-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-siwc-zh-1280-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-siwc-zh-360-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-siwc-zh-360-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-totp-en-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-totp-en-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-totp-zh-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-totp-zh-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-zh-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-zh-1280-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-zh-1920-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-zh-1920-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-zh-360-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-zh-360-light.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-zh-768-dark.png",
    "docs/visual-reviews/assets/2026-11-18-credential-management/updated-zh-768-light.png"
  ],
  "commands": [
    "Playwright Chromium in-memory Vite build of actual TokenGuardV2View, SIWCAccountSettings, OpenAITOTPDialog and shared BaseDialog; synthetic APIs and records; zh/en, light/dark, four widths, reduced-motion, Escape and focus assertions",
    "vitest 8 targeted files / 118 tests",
    "targeted ESLint on changed components, API, locales and tests",
    "pnpm design:check"
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
    "Browser evidence uses synthetic data, mocked APIs, minimal layout, stubbed nested step-up dialog and clipboard mock; it is not production full-page or final user acceptance.",
    "SIWC screenshots render the actual component in shared BaseDialog; EditAccountModal eligibility/preservation is verified by component tests.",
    "Real external worker authorization and user local browser acceptance follow exact-SHA deployment. This task does not start a worker, fetch live credentials or call upstream models."
  ]
}
-->

## Scope

固定上游 v2.21.2 凭据管理接入现有凭证运营和账号编辑区：加密真实就绪状态、外部 local worker、2FA 更换/恢复/敏感导出、Session 导出、SIWC 绑定/解除与显式协议选择。保持管理员入口，不新增用户页面，不自动启动第三方运行时。

## Baseline

基线 6385c8211。实现前检查 TokenGuardV2View、EditAccountModal 的 Excel 区、共享 BaseDialog 和相邻管理表单。baseline 与 prototype 在可见修改前生成，保留原始图片，没有修改后画面冒充基线。

## Prototype

使用固定上游真实凭证组件和现有样式生成原型，明确保留原有管理页面和运行控件。

## Reuse Decision

复用 AppLayout、BaseDialog、Icon、按钮/表单样式、useStepUp、TotpStepUpDialog 和 useClipboard。SIWC 使用账号编辑内的分隔 section，2FA 使用共享弹窗。

## State Coverage

组件测试覆盖加密未配置/就绪/错误、运行控件保存/禁用、SIWC 绑定/轮询/旧响应/危险链接拒绝/解除/错误脱敏、2FA 不确定恢复和 step-up、Session 完成/失败/关闭取消。浏览器实际展示正常凭证页、SIWC 已绑定但未选中及浏览器授权、2FA 不确定恢复。错误和空结果没有冒称浏览器截图证据。

## Viewport Coverage

凭证页四宽度、中英文、浅深色；SIWC 补充 360/1280px 同语言/主题。宽表沿用区域内横向滚动。启用 reduced-motion，验证 Escape 关闭 TOTP 和 SIWC 授权按钮聚焦。

## Evidence

Chromium 渲染真实组件，无开发应用服务器、生产写入、模型调用或真实凭据。API 和账号为合成数据；最外层布局缩减为内容容器，嵌套 step-up 和剪贴板使用 mock。SIWC 使用真实组件和共享弹窗，账号编辑的保留/资格逻辑由组件测试验证。

## Residual Risk

完整集成构建、数据库升级及真实 worker 授权由交付批次汇总。精确部署后的用户本机三身份验收另行记录，合成截图不替代最终生产验收。
