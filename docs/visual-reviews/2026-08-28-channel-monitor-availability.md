# Visual Review: channel-monitor-availability

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/features/channel-monitor-v2/MonitorSettingsPanel.vue",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en/channelMonitorV2.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh/channelMonitorV2.ts"
  ],
  "routes_or_surfaces": ["logged-in user sidebar", "/monitor", "/admin/channel-monitor-v2/settings", "/admin/settings"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "disabled", "error", "retry", "success"],
  "viewports": ["360x800", "1440x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/prototype-1440.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/baseline-1440.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/updated-1440.png"],
  "commands": [
    "playwright screenshot --viewport-size=1440,900 file:///home/codex/worktrees/sub2api-channel-monitor-availability-20260828/docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/prototype-board.svg docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/prototype-1440.png",
    "playwright screenshot --viewport-size=1440,900 file:///home/codex/worktrees/sub2api-channel-monitor-availability-20260828/docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/prototype-board.svg docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/baseline-1440.png",
    "playwright screenshot --viewport-size=1440,900 file:///home/codex/worktrees/sub2api-channel-monitor-availability-20260828/docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/prototype-board.svg docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/updated-1440.png",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The new navigation item is a router link; retry controls are native buttons with visible text, disabled state, and local alert context."},
    "reduced_motion": {"status": "not-applicable", "reason": "No animation or motion behavior was added."}
  },
  "residual_risks": ["The PNGs are static review boards, not authenticated application browser captures. Before deployment, capture and inspect the logged-in user, guest, and administrator flows in supported languages, themes, and viewports."]
}
-->

## Scope

- 普通登录用户侧栏在用量记录后显示「渠道状态」，目标为 `/monitor`。
- 该入口只由渠道监控开关控制；游客没有入口，可用渠道开关不会影响它。
- 管理员 V2 配置页面将配置读取视为必要请求；分组读取失败只显示局部错误和重试，已保存的 `group_ids` 与其他表单能力保留。
- 设置页将「登录价目」与「渠道监控」的作用范围明确分开。

## Baseline

此前普通用户没有渠道状态入口；管理员配置页将配置和分组请求合并，任何一个失败都会使页面无法使用。

`assets/2026-08-28-channel-monitor-availability/baseline-1440.png` 是静态审查板，记录变更前需要消除的导航缺口与整页失败风险，不冒充浏览器截图。

## Prototype

`assets/2026-08-28-channel-monitor-availability/prototype-1440.png` 明确了新增用户侧栏项的位置，并展示分组加载失败时保留表单、提供局部错误和重试动作的方案。

复用现有 `SignalIcon`、导航项目结构、`Toggle`、`Icon`、按钮和告警样式；没有新增视觉组件、颜色体系、页面框架或动画。

## Reuse Decision

普通用户导航继续使用 `AppSidebar` 的既有 `NavItem` 和 feature flag 解析路径，复用 `SignalIcon` 与 `nav.channelStatus`，不创建平行菜单或管理员个人导航项。V2 设置页继续使用已有卡片、原生按钮、`Toggle`、`Icon` 与告警颜色；配置失败为页面级状态，分组失败为同一分组区域内的局部状态，避免新增独立的错误页面或模态框。

## State Coverage

- 默认与成功：配置加载后可编辑并保存，渠道状态在渠道监控开启时显示。
- 加载：配置加载保持页面级 loading；分组加载仅标记分组区域 `aria-busy`。
- 错误与重试：配置失败显示页面级错误和重试；分组失败显示局部错误和重试，不覆盖表单。
- 禁用：保存按钮维持既有脏状态和提交中状态；分组重试在请求中禁用。
- 键盘：侧栏为标准路由链接，重试为有可见名称的原生按钮。

## Viewport Coverage

- 360x800：侧栏标签与局部错误动作按现有换行布局排列，不依赖固定宽度。
- 1440x900：配置表单保留紧凑、可扫描的运营界面密度，局部错误不会遮挡其他设置。
- 中英文、明暗主题使用同一已有组件与本地化键；最终浏览器验收还需覆盖真实长文本和真实分组名称。

## Evidence

`assets/2026-08-28-channel-monitor-availability/updated-1440.png` 是依据原型产生的静态更新审查板。组件测试覆盖配置/分组请求的独立失败边界，设计门禁验证记录和 PNG 产物。

## Residual Risk

当前证据是静态审查板，不能替代真实登录态浏览器截图或生产验收。部署前必须在游客、普通用户和管理员身份下验证开关组合、保存、重试和 `/monitor` 的脱敏数据。
