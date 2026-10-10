# 签到读取状态与玩法中枢刷新

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/CheckInView.vue",
    "frontend/src/views/user/PlayHubView.vue",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts"
  ],
  "routes_or_surfaces": [
    "/check-in",
    "/play"
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
    "docs/visual-reviews/assets/checkin-status/prototype-error.png",
    "docs/visual-reviews/assets/checkin-status/prototype-milestone.png",
    "docs/visual-reviews/assets/checkin-status/prototype-hub.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/checkin-status/baseline-503.png",
    "docs/visual-reviews/assets/checkin-status/baseline-milestone.png",
    "docs/visual-reviews/assets/checkin-status/baseline-hub.png",
    "docs/visual-reviews/assets/checkin-status/hub-session-repro-hub-session.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/checkin-status/updated-1280-zh-light-checkin-ready.png",
    "docs/visual-reviews/assets/checkin-status/updated-360-zh-dark-checkin-error.png",
    "docs/visual-reviews/assets/checkin-status/updated-768-en-light-hub-loading.png",
    "docs/visual-reviews/assets/checkin-status/updated-1920-en-dark-hub-ready.png",
    "docs/visual-reviews/assets/checkin-status/updated-navigation-success.png",
    "docs/visual-reviews/assets/checkin-status/updated-hub-session.png",
    "docs/visual-reviews/assets/checkin-status/updated-hub-logout.png"
  ],
  "commands": [
    "pnpm --dir frontend build",
    "python3 docs/archive/completed-tasks/2026-10-checkin-status/browser.py --phase updated --dist backend/internal/web/dist",
    "./scripts/test-checkin-contract.sh"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "本批未新增或更改动画；浏览器启用reduce，截图完成既有入场动画，未将全站动效策略作为本批通过项。"
    }
  },
  "residual_risks": [
    "仅隔离构建资源与HTTP夹具；PG另由真实HTTP合同验证。不是生产或用户本机三身份验收。",
    "其他福利页未完成本批跨层/视觉验收。"
  ]
}
-->

## Scope

仅两个页面读取状态、刷新入口和剩余里程碑文案；不改奖励、路由、布局规范或全站设计。用户已授权窄批错误状态修复。

## Baseline

#353 merge 基线。GET503显示关闭，6/7天显示再签7天；中枢已有error/retry样式，首次失败重复重试守卫不成立。已读本页、相邻BlindboxView、共享AppLayout、Toast、按钮样式及设计规范。

## Prototype

实现前真实构建截图并用DOM预览同款secondary刷新/重试按钮及剩余1天。prototype不是实际回归证据。错误/加载时余额用 —，避免把旧缓存显示为当前值；已在实现前更新原型。503最终文字使用“稍后重试”，另为网络异常提供“检查连接”文案，避免将服务故障归咎网络。

## Reuse Decision

原有gw-panel/gw-subtitle/gw-btn-secondary及common.refresh/retry，无新样式或例外。错误role=alert，页面aria-busy，原生按钮disabled并在处理函数防重；中文英文同义。

## State Coverage

详见同批browser.py与updated-browser.json：加载、关闭、503、403、网络、重试、重开、焦点/hover/键盘、重复刷新、离开重开后旧响应、真实路由签到→中枢→签到的成功恢复。PG HTTP测试另验证跨层持久化与拒绝/防重奖。补丁后新增同一 Hub DOM 组件 A→B（实际 login HTTP）、token 采用以及登出时的迟到响应回归；旧构建的 A$1.00 RED 与新构建 B$2.00/登出空面板 PNG 均已人工复核。

## Viewport Coverage

360中文深色、768英文浅色、1280中文浅色、1920英文深色，两页均覆盖。reduced-motion启用；无新增动画。200%缩放未测，不记通过。

## Evidence

浏览器报告/构建哈希与门禁结果见 docs/archive/completed-tasks/2026-10-checkin-status/。会话隔离补丁后的最终生产构建矩阵 14/14 通过；8个尺寸/语言/主题页面组合的JS异常与横向溢出均为0。本批还增加另一客户端发奖后的余额刷新，以及旧资料响应/身份切换与 token 续期并发的真实 HTTP 回归。截图固定完成既有有限入场动画，避免把动画中间帧误当成颜色问题；没有修改共享样式。

## Residual Risk

服务器技术验证不能替代生产与用户本机验收；本批不合并或部署。答题/盲盒下一批计划与已有证据盘点在同批README中。
