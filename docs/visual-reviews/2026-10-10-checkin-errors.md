# 签到错误契约回归

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/CheckInView.vue"
  ],
  "routes_or_surfaces": [
    "/check-in"
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
    "docs/visual-reviews/assets/checkin-errors/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/checkin-errors/baseline-1280-zh-light-duplicate-before.png",
    "docs/visual-reviews/assets/checkin-errors/baseline-1280-zh-light-duplicate.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/checkin-errors/updated-1280-zh-light-duplicate.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-dark-duplicate.png",
    "docs/visual-reviews/assets/checkin-errors/updated-768-en-light-duplicate.png",
    "docs/visual-reviews/assets/checkin-errors/updated-1920-en-dark-duplicate.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-makeup.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-qualification.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-success.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-loading.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-empty.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-load-error.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-pending-submitting.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-pending.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-pending-makeup-submitting.png",
    "docs/visual-reviews/assets/checkin-errors/updated-360-zh-light-pending-makeup.png"
  ],
  "commands": [
    "pnpm --dir frontend build",
    "python docs/archive/completed-tasks/2026-10-checkin-errors/browser.py --phase baseline",
    "python docs/archive/completed-tasks/2026-10-checkin-errors/browser.py --phase updated",
    "./scripts/test-checkin-contract.sh"
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
    "浏览器使用实际 production build 与回环 HTTP 故障夹具；不等于生产验收。PG 持久化由独立 HTTP 合同测试验证。",
    "既存状态读取 503 被显示为功能未开启，留待独立错误/空状态修复。",
    "未覆盖其他福利页面全状态；未进行生产或用户本机三身份验收。"
  ]
}
-->

## Scope

仅修复普通签到/补签从真实客户端提取业务错误，以及补签拒绝后刷新过期状态。没有模板、布局、样式、奖励规则或价格改动。现有中文提示与英文资源均复用。

## Baseline

基线为 `726289bdb757e572eac10ab5f12c689c4dc087b5`。两个 catch 读取 `response.data.code`，实际 Axios 输出顶层数字 `code` 与字符串 `reason`。普通签到 409 显示通用失败且不刷新；补签拒绝后旧按钮保留。

已阅读本页、相邻 `PlayHubView`、`BlindboxView` 的错误工具用法、共享 `Toast`、AppLayout 和 growth-world 样式。截图通过未改实现的生产构建记录。

## Prototype

`assets/checkin-errors/prototype-1280.png` 在实现前由未修改页面、已签到状态和共享 Toast 生成，预览“今日已签到”提示及禁用按钮；它是原型状态演示，不作为修复通过证据。用户已授权错误契约修复并明确要求保留现有风格，本图不扩大边界。

## Reuse Decision

复用 `extractApiErrorCode`，以业务 `reason` 优先；不改公共客户端。复用现有 `checkin.*` 文案、Toast、按钮、AppLayout 与页面结构，无新视觉模式或治理例外。

## State Coverage

- default / hover / focus-visible / active：真实构建按钮经 Tab 到达、hover、Enter 提交。
- loading / disabled：状态读取保持加载；真实构建在普通签到/补签 POST 保持 pending 时用鼠标双击及键盘重复提交，均只产生一次请求，拒绝后恢复；真实 Axios 超时另由组件 HTTP 测试验证。
- error：重复签到、补签不可用、资格服务不可用；HTTP 测试另覆盖现有所有业务分支、未知错误、实际超时。
- success：现金及七天里程碑显示；真实 PG 测试另覆盖成长能量、现金、里程碑和补签，刷新读回。
- empty：功能关闭显示现有禁用文案。读取 503 同样被显示为关闭是既存缺口，明确留待后续，未称通过。

## Viewport Coverage

360 中文深色、768 英文浅色、1280 中文浅色、1920 英文深色；另外 360 中文浅色覆盖加载、空、读取错误、提交错误、补签和成功。脚本核对文档无横向溢出、无页面 JavaScript 异常。

本批没有几何、排版或动画变化；所有浏览器实例启用 reduced-motion，200% 缩放没有重测，不记为通过。

## Evidence

更新构建浏览器矩阵 12/12 通过，页面 JavaScript 异常为 0，横向溢出为 0。

可复跑脚本及逐例报告位于 `docs/archive/completed-tasks/2026-10-checkin-errors/`。基线、原型与更新图片均为可解码 Chromium 截图。故障夹具只绑定回环地址并拦截外部浏览器请求；不使用真实账号或生产端点。

## Residual Risk

本记录是服务器技术证据。草稿 PR 不合并部署；用户本机、普通用户/管理员生产验收尚未进行。其他福利入口没有因签到通过而被宣布完成。
