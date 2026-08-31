<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/play.ts",
    "frontend/src/views/user/PlayHubView.vue",
    "frontend/src/views/user/CheckInView.vue",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/views/public/__tests__/BlindboxView.spec.ts",
    "frontend/src/views/public/QuizQuestView.vue",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts"
  ],
  "routes_or_surfaces": ["/check-in", "/quiz-quest", "/blindbox"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["explorer", "active", "loading", "disabled", "empty", "error", "success", "already submitted", "reward locked"],
  "viewports": ["360x800", "768x1024", "1280x900", "1600x1000"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v182-growth-qualification/prototype-growth-qualification.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/play-growth-competition/prototype-team-arena-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v182-growth-qualification/prototype-growth-qualification.png"],
  "commands": ["firefox --headless --screenshot", "pnpm design:check", "pnpm typecheck", "pnpm lint:check"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Existing native buttons, radios and links retain focus-visible behavior."},
    "reduced_motion": {"status": "passed", "notes": "No new continuous animation is introduced; existing progress transition is bounded."},
    "copy_locale": {"status": "passed", "notes": "Explorer and active labels are served from paired locale resources."}
  },
  "residual_risks": ["Static review board is not a live authenticated browser capture; final user and operator acceptance must verify real qualification signals and long localized copy at all listed viewports."]
}
-->

## Scope

签到和答题继续允许参与，但将可变现福利限制在后端统一评估的 active 档位。explorer 档位显示成长能量、达标进度和恢复条件；不显示余额、优惠券或兑换码奖励承诺。盲盒是余额消耗后的可变现开奖，因此 explorer 档仅显示服务端资格原因并禁用开启，不扣余额、不抽奖，也不生成成长能量。

## Baseline

既有 Play 页面只展示余额奖励或“奖池未配置”，无法区分参与资格，也不能解释为什么奖励模式不同。页面复用现有 `AppLayout`、`AuthenticatedPlayShell`、Play 按钮和状态 token。

## Prototype

`docs/visual-reviews/assets/v182-growth-qualification/prototype-growth-qualification.png` 是真实 PNG 原型，展示同一壳层下的 explorer/active 两种状态、步骤进度、成长能量和答题成功路径。

## 2026-08-30 Delta Review

本轮实现新增了服务端 `growth_eligibility.progress` 契约，原型中的“邮箱验证 / 注册满 3 天 /
真实使用”三段进度与接口字段保持一致；Explorer 的签到、答题和盲盒状态只展示成长能量与达标
路径，Active 才展示可兑换福利。真实 PNG 已重新解码检查，未引入新的视觉体系或持续动画。

本轮同时复核了奖励动作的事务顺序：先记录唯一动作，再写资格快照并关联 FK，最后才发券、发码
或记余额。视觉板只证明信息层级和长文案布局，不替代真实 API、数据库回滚、暗色主题、200% 缩放
及生产身份验收。

## Reuse Decision

签到复用现有 `AppLayout`、`growth-world.css`、语义状态 token 和原生按钮；答题复用
`AuthenticatedPlayShell`、公共页面工具栏、既有答题选项和 `CouponRewardCard`。资格说明只增加
后端返回的状态文案和进度，不新增一套卡片、图标或动画系统。

## State Coverage

默认、hover、active、focus-visible、loading、disabled、empty、error、success 和已完成状态均
沿用现有控件尺寸与焦点轮廓。explorer/active 只切换奖励模式和达标说明；按钮不因长中文或英文文案
改变布局，答题提交仍由服务端幂等约束保护。

## Viewport Coverage

原型与审查矩阵覆盖 `360x800`、`768x1024`、`1280x900` 和 `1600x1000`，并要求中英文、浅深色、
200% 缩放、键盘导航和 reduced-motion 复核。静态板不能证明真实 API 数据密度或生产浏览器的换行，
这些仍是部署后的用户本地验收项。

## State Matrix

| 状态 | 签到 | 答题 | 用户可见下一步 |
| --- | --- | --- | --- |
| explorer | 允许，记录成长能量 | 允许，记录成长能量 | 盲盒显示资格原因且不可开启；验证邮箱、注册满 3 天，并完成真实调用/充值/订阅之一 |
| active | 按已发布奖池开奖 | 按已发布奖池开奖 | 继续使用，保持资格 |
| loading/error/disabled | 稳定状态区和重试/说明 | 稳定状态区和重试/说明 | 不改变按钮尺寸 |
| success/already submitted | 就地保留结果 | 就地保留分数和奖励模式 | 不重复记账 |

## Evidence

原型使用 Firefox headless 生成并通过 PNG 解码检查。浏览器真实 API 数据、暗色主题、200% 缩放和生产身份验收仍是发布门禁，不由静态板替代。

## Residual Risk

补签只对 active 发放可变现奖励；这条策略需要运营确认后再启用线上补签开关。生产当前签到和答题开关均为关闭，不能在上线说明中描述为已对用户发放。
