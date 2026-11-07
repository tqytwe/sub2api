# 渠道状态卡片色条修复

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/features/channel-monitor-v2/MonitorStatusCards.vue",
    "frontend/src/features/channel-monitor-v2/monitorCards.ts"
  ],
  "routes_or_surfaces": ["/monitor V2 cards, shared administrator V2 card preview"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en/light", "en/dark"],
  "states": ["healthy", "warning", "critical", "unknown", "low volume", "no samples", "redacted counts", "samples timeline", "windows timeline"],
  "viewports": ["360x1000", "768x1000", "1280x1000", "1920x1000"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/monitor-status-bars/prototype-status-bars.png"],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/monitor-status-bars/baseline-user-card-crop.png",
    "docs/visual-reviews/assets/monitor-status-bars/baseline-1280-light.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/monitor-status-bars/updated-360-light.png",
    "docs/visual-reviews/assets/monitor-status-bars/updated-360-dark.png",
    "docs/visual-reviews/assets/monitor-status-bars/updated-1280-light.png",
    "docs/visual-reviews/assets/monitor-status-bars/updated-1280-dark.png"
  ],
  "commands": [
    "MONITOR_PLAYWRIGHT_MODULE=/path/to/playwright-core/index.mjs node docs/archive/completed-tasks/2026-11-07-monitor-status-bars/verify-render.mjs --baseline",
    "MONITOR_PLAYWRIGHT_MODULE=/path/to/playwright-core/index.mjs node docs/archive/completed-tasks/2026-11-07-monitor-status-bars/verify-render.mjs",
    "cd frontend && pnpm exec vitest run src/features/channel-monitor-v2/__tests__/monitorCards.spec.ts src/features/channel-monitor-v2/__tests__/MonitorStatusCards.pelican.spec.ts",
    "make test",
    "make build",
    "./scripts/check-fork-integrity.sh"
  ],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "Read-only bars and decorative status dots; existing title/text labels remain and no focusable control changes."},
    "reduced_motion": {"status": "passed", "reason": "Geometry checks run with reduced-motion enabled; no animation is added."}
  },
  "residual_risks": ["Browser captures render the actual compiled Vue component and production Tailwind CSS with synthetic data and only the closed Pelican dialog stubbed. They do not show the authenticated production route. User-local browser acceptance after deployment remains pending; no local Chrome plugin is available in this server session."]
}
-->

## Scope

仅修复现有卡片样本条高度、异常/未知状态点颜色及脱敏计数下的状态保留。不新增导航或控件。
管理员复用同一卡片组件；不变更后端评分、隐私规则、采样、鹈鹕、质量运维或玩法福利。

## Baseline

已查看用户提供的生产截图，并只保存卡片区域裁图，排除账号、头像和余额。
编译基线 `01fc395e0306b7af031643e3412c41979afec4be` 后真实浏览器测量证实空 span 高度为 0；
异常及未知点引用的两个 CSS 变量均未定义。相邻 `RelayPulseMatrix.vue` 使用已有
`--monitor-health-0` 和 `--border-strong`，本修复复用这些 token。

## Prototype

`assets/monitor-status-bars/prototype-status-bars.png` 在修改组件前生成并展示。
这是标明示例数据的静态原型；用户已授权修复的边界为恢复原状态指示，不重新设计卡片。

## Reuse Decision

沿用既有布局、文字徽标、颜色映射、样本顺序、tooltip 和 18 槽上限。
使用父级现有 20px 高度；状态点复用相邻矩阵已有 token，不新建全局样式或组件。

## State Coverage

健康绿、需关注黄、异常红、未知灰分别检查非透明且互异；色条高 20px。
有样本但计数为脱敏 0 时保留后端状态（包括 unknown）；可见的真实低流量样本继续为未知。
无样本继续显示原空状态，窗口模式保留空槽。默认/成功即上述只读状态；loading/error 容器不变，
hover 仅原生 title，active/focus/disabled 不适用于非交互色条。

## Viewport Coverage

360、768、1280、1920px × 浅/深色 × 中/英文 × samples/windows，共 32 个组件场景。
检查所有色条几何尺寸、状态点颜色、无意横向溢出、页面运行错误；保存移动及桌面浅深色截图。
这些是组件夹具截图，不是生产完整页面或本地浏览器验收。

## Evidence

可复跑脚本、基线和更新后几何结果随修复归档；脚本离线拦截所有浏览器请求，不启动应用服务。
基线模式从固定 Git SHA 读取原组件和算法，避免把更新实现当作基线。

## Residual Risk

当前服务器会话没有用户本机浏览器插件。部署后仍须用户本机浏览器核验游客权限边界、普通用户
`/monitor`、管理员共享卡片与浅深色。自动化和截图不替代这些生产验收。
