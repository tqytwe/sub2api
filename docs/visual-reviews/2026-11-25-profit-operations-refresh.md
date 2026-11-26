# 利润报表手动刷新绕过缓存

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/operations/ProfitAnalytics.vue",
    "frontend/src/api/admin/profitOperations.ts",
    "frontend/src/api/__tests__/profitOperations.refresh.spec.ts"
  ],
  "routes_or_surfaces": ["/admin/channels/operations 利润报表手动刷新按钮"],
  "languages_and_themes": ["zh/light", "en/dark"],
  "states": ["default", "focus-visible", "loading-disabled", "success", "error"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-19-channel-profit/prototype-1280-light-default.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-19-channel-profit/baseline-1280-light-default.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-1280-light-stateloading.png",
    "docs/visual-reviews/assets/2026-11-19-channel-profit/updated-1920-dark-default.png"
  ],
  "commands": [
    "Reused prior Chromium browser screenshots for the visually unchanged report surface; pnpm test --run src/api/__tests__/profitOperations.refresh.spec.ts",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "现有按钮保留原生键盘和 focus-visible 行为；刷新请求期间保持 disabled。"},
    "reduced_motion": {"status": "passed", "notes": "本改动不新增动画或运动效果。"}
  },
  "residual_risks": ["本次只改变刷新请求是否绕过服务端报告缓存，页面布局和样式不变；真实部署后的管理员验收仍待完成。"]
}
-->

## Scope

手动刷新现在显式请求服务端重算利润报表；首次进入、筛选、翻页等普通读取仍可复用缓存。
刷新按钮没有新增或调整可见样式。

## Baseline

利润报表现有 360、768、1280 和 1920 像素截图显示，刷新按钮布局、尺寸、文案与其他加载态不变。

## Prototype

沿用 2026-11-19 利润报表的真实 Chromium 原型图。该次原型不涉及本次新增的缓存绕过行为，但按钮外观没有修改。

## Reuse Decision

沿用 2026-11-19 利润报表的真实 Chromium 原型、基线及构建后截图。该改动只变更按钮调用语义，DOM、布局、文案、颜色与状态样式没有变化，因此复用同一报表界面的既有截图作为像素级视觉参照；没有把静态审查板冒充为浏览器截图。

## State Coverage

普通读取不带刷新参数，管理员显式刷新发送 `refresh_report=1`。刷新时的 disabled 状态及请求成功/失败反馈沿用现有组件行为。API 测试分别断言普通读取保留缓存路径、手动刷新发送绕过参数；此记录不替代部署后的管理员实际操作验收。

## Viewport Coverage

## Evidence

复用的真实 Chromium 截图位于 `docs/visual-reviews/assets/2026-11-19-channel-profit/`。API 单测覆盖普通读取和显式刷新参数；当前工作不生成不同的像素结果。

## Residual Risk

视口与主题复用同一报表组件的既有截图覆盖（360、768、1280、1920；中英文、浅色、深色）。因为没有界面结构或样式变化，不产生新增断点布局。生产登录后的真实数据与缓存失效仍须在部署验收时确认。
