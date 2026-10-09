# TPS 帮助提示的滚动定位修复

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/components/common/HelpTooltip.vue"],
  "routes_or_surfaces": ["/usage", "/admin/usage", "shared HelpTooltip viewport positioning"],
  "languages_and_themes": ["en/light", "zh/dark"],
  "states": ["scrolled document Enter open", "open during document and nested scroll", "viewport resize", "left/right/top/bottom boundaries", "Escape and retained trigger focus", "existing hover and outside/close-button dismissal"],
  "viewports": ["1280x900", "1920x900", "960x900"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/v2103-tooltip-position/baseline-scroll-1280.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/v2103-tooltip-position/prototype-scroll-1280.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v2103-tooltip-position/updated-user-scroll-1280.png",
    "docs/visual-reviews/assets/v2103-tooltip-position/updated-admin-scroll-1920.png",
    "docs/visual-reviews/assets/v2103-tooltip-position/updated-top-edge-960.png",
    "docs/visual-reviews/assets/v2103-tooltip-position/updated-right-edge-960.png"
  ],
  "commands": [
    "vitest run src/components/common/__tests__/HelpTooltip.spec.ts src/utils/__tests__/usageTps.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts src/views/user/__tests__/UsageView.spec.ts src/views/admin/__tests__/UsageView.spec.ts",
    "Playwright Chromium in-memory real UsageTable and HelpTooltip fixture; real window.scrollTo, inner scroll container, setViewportSize, Enter/Escape; viewport bounds and elementFromPoint assertions",
    "pnpm design:check"
  ],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": [
    "Browser fixtures use synthetic records and are not production or full-page final acceptance.",
    "Independent reviewers must recheck the new PR head on the full user/admin pages.",
    "Mobile/Canvas remain deferred; this change does not redesign those surfaces."
  ]
}
-->

## Scope

针对 PR #338 独立审查的 P2：页面滚动后 TPS 提示框跑出视口。只修复共享 HelpTooltip 的坐标和边缘定位，保留触发、关闭、主题及文字样式；不更改 TPS、权限、费用或其他业务功能。用户已明确授权此边界。

## Baseline

基线 commit `0462fe370cf862453c6261218a55ff61e0d3d966`。先读取 HelpTooltip、UsageTable、既有 hover/click 测试和前端规范。独立审查在完整用户页与管理员页发现异常；本环境的真实组件复现中，window.scrollY=870、按钮 top=597，提示框 top=1298.5，超出 900px 视口。坐标证据见 `assets/v2103-tooltip-position/baseline-metrics.json`。

## Prototype

实现前保存旧组件的浏览器基线，并通过 DOM 定位提案生成 `prototype-scroll-1280.png`：提示框使用视口坐标，显示在按钮上方。原型仅用于确认定位结果，不冒充修复后的组件。

## Reuse Decision

继续使用共享 HelpTooltip，不为 TPS 新建弹层。fixed 定位与 getBoundingClientRect 使用同一视口坐标，不再叠加 scrollX/Y；按实际弹层尺寸保留 8px 边缘，上方空间不足时向下显示，箭头与悬停通路随方向调整。窗口 resize、文档及嵌套容器滚动继续复用既有监听器。

## State Coverage

新增 3 个先失败后通过的单测覆盖滚动偏移、捕获滚动、resize、左右边缘、顶部翻转、底部定位及 Escape 焦点保留。既有 hover 往返、click 切换、外部点击和关闭按钮测试继续通过。定向范围共 81 项测试。

18 项真实浏览器断言覆盖两种身份的共享表、中英文/浅深色、Enter 打开、保持打开时文档滚动 80px、内层滚动 24px、resize 到 960px、四边定位与 Escape 后焦点保留。每项同时断言提示框在视口内及中心点命中真实提示框，结果见 `assets/v2103-tooltip-position/browser-checks.json`。修复后用户场景 top=428.5、管理员场景 top=490，均可见。

提示框无保存流程，loading/empty/error/success/disabled 不新增状态，沿用已有 TPS 批次证据；默认 hover 和 click 行为由回归测试保护。

## Viewport Coverage

1280×900 用户场景、1920×900 管理员场景及两侧 resize 至 960×900。启用 reduced-motion。移动端/Canvas 按用户范围仍不处理。

## Evidence

本记录的基线、原型和修改后 PNG 均来自 Chromium 实际渲染。未启动应用服务；浏览器在内存中加载生产模式构建的真实组件与合成数据。原始完整页面异常由独立审查提供，修复后的完整页复审需要关联新 SHA。

## Residual Risk

等待独立审查对新 head 的完整页复审，以及新提交的全部 CI。生产部署、用户本机最终验收未执行；旧 head 的 CI 通过不能替代新提交的验证。
