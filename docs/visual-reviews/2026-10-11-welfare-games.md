# 答题与盲盒福利闭环

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/public/QuizQuestView.vue",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts"
  ],
  "routes_or_surfaces": [
    "/quiz-quest",
    "/blindbox"
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
    "docs/visual-reviews/assets/welfare-games/prototype-quiz-error.png",
    "docs/visual-reviews/assets/welfare-games/prototype-blindbox-recovery-and-weights.png",
    "docs/visual-reviews/assets/welfare-games/prototype-quiz-guest.png",
    "docs/visual-reviews/assets/welfare-games/prototype-quiz-redeem-toast.png",
    "docs/visual-reviews/assets/welfare-games/prototype-quiz-recovered-balance.png",
    "docs/visual-reviews/assets/welfare-games/prototype-blindbox-logout-cleared.png",
    "docs/visual-reviews/assets/welfare-games/prototype-blindbox-switch-cleared.png",
    "docs/visual-reviews/assets/welfare-games/prototype-blindbox-failed-qualification-hidden.png",
    "docs/visual-reviews/assets/welfare-games/prototype-blindbox-settled-qualification-hidden.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/welfare-games/baseline-quiz-503.png",
    "docs/visual-reviews/assets/welfare-games/baseline-blindbox-unknown-and-weights.png",
    "docs/visual-reviews/assets/welfare-games/baseline-quiz-redeem-toast.png",
    "docs/visual-reviews/assets/welfare-games/baseline-quiz-stale-balance.png",
    "docs/visual-reviews/assets/welfare-games/baseline-blindbox-logout-reward.png",
    "docs/visual-reviews/assets/welfare-games/baseline-blindbox-switch-prize.png",
    "docs/visual-reviews/assets/welfare-games/baseline-blindbox-failed-qualification-feed.png",
    "docs/visual-reviews/assets/welfare-games/baseline-blindbox-settled-qualification-feed.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-1280-zh-light-error.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-1280-zh-light-loading.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-1280-zh-light-ready.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-1280-zh-light-success.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-1920-en-dark-error.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-1920-en-dark-loading.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-1920-en-dark-ready.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-1920-en-dark-success.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-360-zh-dark-error.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-360-zh-dark-loading.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-360-zh-dark-ready.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-360-zh-dark-success.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-768-en-light-error.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-768-en-light-loading.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-768-en-light-ready.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-768-en-light-success.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-disabled.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-explorer.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-guest.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-logout-cleared.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-qualified-recent-restored.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-recovered-reward.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-settled-qualification-failed.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-settled-qualification-retried.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-switch-cleared.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-switch-failed-qualification.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-switch-qualification-retried.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-unknown-recovery.png",
    "docs/visual-reviews/assets/welfare-games/updated-blindbox-unpublished.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-1280-zh-light-error.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-1280-zh-light-loading.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-1280-zh-light-ready.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-1280-zh-light-success.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-1920-en-dark-error.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-1920-en-dark-loading.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-1920-en-dark-ready.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-1920-en-dark-success.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-360-zh-dark-error.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-360-zh-dark-loading.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-360-zh-dark-ready.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-360-zh-dark-success.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-768-en-light-error.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-768-en-light-loading.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-768-en-light-ready.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-768-en-light-success.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-disabled.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-duplicate-balance.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-empty.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-guest.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-redeem-toast.png",
    "docs/visual-reviews/assets/welfare-games/updated-quiz-unknown-balance.png"
  ],
  "commands": [
    "pnpm --dir frontend build",
    "python3 docs/archive/completed-tasks/2026-10-welfare-games/browser.py --phase updated --dist backend/internal/web/dist",
    "python3 docs/archive/completed-tasks/2026-10-welfare-games/browser.py --phase blindbox-updated --dist backend/internal/web/dist",
    "./scripts/test-welfare-contract.sh"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "本批无新动画；浏览器启用 reduce，不将全站动画策略记为通过。"
    }
  },
  "residual_risks": [
    "实际Vite资源搭配合成HTTP fixture；真实PG由HTTP/Vue合同单独验证，不声称浏览器→PG一体化或生产JWT验收。",
    "未合并部署；用户本机三身份、运营页、200%缩放及真正午夜时钟切换未测。"
  ]
}
-->

## Scope

答题读取错误/重试/刷新、游客登录、空题与正确奖种提示、未知结果余额同步；盲盒已发布概率、原请求恢复和身份切换状态隔离。复用既有布局与样式，不改成本、奖种、规则、路由或共享鉴权。

## Baseline

基线 c939d570 与 #355 构建树一致。实际构建复现答题503误显关闭、兑换码Toast误报现金及409后旧余额；盲盒末次未知结果无法恢复、概率未公开。审查还以生产Pinia.logout/checkAuth及合成HTTP身份复现登出残留奖励、切号旧奖池。

## Prototype

每次可见修复前先生成基线截图和DOM预览图片，复用play-note、play-btn-secondary、登录入口、页头金额和已有奖励弹层。原型不冒充回归。身份原型仅移除旧内容，无新视觉组件。

## Reuse Decision

复用原有shell、按钮、CouponRewardCard、Toast、Dialog及中英词条；无CSS/图标/共享组件变化。错误role=alert，答题aria-busy，原生disabled与处理函数防重。

## State Coverage

答题：503→重试、读取期间禁用/隐藏旧题、连续刷新单GET、hover/focus与键盘提交、成功重载、空题/关闭/游客零私有请求、兑换码Toast、409/断连接后页头余额无需reload即可同步，同tick提交→刷新只1POST+1权威GET。

盲盒：503→重试、加载/默认/hover/focus与键盘提交、0/100/0已知概率、兑换码成功与余额；末次事务后断连接→重挂载同key恢复；关闭/未发布/Explorer/游客，游客零私有statusGET。登出奖励消失；B身份GET等待中无A奖池；Explorer不新增近期读取，合格身份恢复非空近期记录。近期空列表原状态由早期fixture及既有测试覆盖，不冒充新增生产数据。

## Viewport Coverage

360中文深色、768英文浅色、1280中文浅色、1920英文深色；各组检查无横向溢出、无JS异常。启用reduced-motion，不将全站动画策略记为通过。200%缩放未测。

## Evidence

最终截图和报告来自冻结后构建。对应脚本、来源/日志哈希、独立审查与完整门禁见[技术记录](../archive/completed-tasks/2026-10-welfare-games/README.md)。quiz-verification.json保留答题子任务先前有效构建证据；整批最终构建归属见verification.json及两个browser.json，不混用历史截图归属。

## Residual Risk

无生产访问、合并或部署。服务器浏览器不能替代用户本机验收。仅验证前一日历史不占当前日次数，未推进真实服务器时钟跨午夜。组队、擂台、推广、券包和运营页未按本批标准验收。

质量补证：资格GET失败时不读登录用户的中奖动态，重试成功后恢复；先实际构建截图与DOM原型，正式HTTP mount/switch两项RED后GREEN。该轮构建d5657790为初始PR历史证据；最终根级补证另见下文。

根级审查补证：同账号结算后的资格GET失败仍保留已展示/迟到的公开匿名化中奖动态。先实际构建baseline/prototype，后真实Axios已显示与延迟乱序RED。修复仅使资格失败时清空动态/旧动态错误并失效在途代次；属于页面状态一致性，不是已证实私有泄漏，也不改变公开接口规则。新版构建 index `fbff6b8fd39679d7c454486a0131ec0e343d3871c001926ad79d7e7285962e61` 已验证盲盒8组：新增资格失败空动态及重试恢复截图，真实HTTP另覆盖旧响应在重试前/后到达。
