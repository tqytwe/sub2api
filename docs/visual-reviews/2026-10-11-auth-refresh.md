# 答题原生续期保留选择

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/public/QuizQuestView.vue"
  ],
  "routes_or_surfaces": [
    "/quiz-quest"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en/light",
    "en/dark"
  ],
  "states": [
    "default",
    "active",
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
    "docs/visual-reviews/assets/auth-refresh/prototype-quiz-selected.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/auth-refresh/baseline-quiz-selection-lost.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/auth-refresh/updated-360-zh-dark/reading-after.png",
    "docs/visual-reviews/assets/auth-refresh/updated-768-en-light/reading-after.png",
    "docs/visual-reviews/assets/auth-refresh/updated-1280-zh-light/reading-after.png",
    "docs/visual-reviews/assets/auth-refresh/updated-1920-en-dark/reading-after.png"
  ],
  "commands": [
    "python docs/archive/completed-tasks/2026-10-auth-refresh/browser-cross-tab.py --dist backend/internal/web/dist --out docs/visual-reviews/assets/auth-refresh/updated-1280-zh-light --cases reading --assert-safe",
    "pnpm --dir frontend design:check"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "本批只改变同会话续期时保留已选答案的watcher，复用原单选控件与焦点样式，未增改键盘路径；不冒称完整键盘验收。"
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "无动画改动；实际构建截图启用reduce，不作为全站动效验收。"
    }
  },
  "residual_risks": [
    "截图来自实际Vite构建及回环HTTP合成fixture，不代表生产或浏览器到PG一体化验收。",
    "本批不修改布局、样式或中文文案；用户本机三身份验收仍待后续部署。"
  ]
}
-->

## Scope

只改 QuizQuestView 的会话 watcher：同一账户、同一生命周期的正常令牌旋转保留选择，换号/退出/同账户重新登录按原流程清理。普通用户答题页；四视口、中英文和明暗主题。无模板、CSS、文案、奖励或规则变更。

## Baseline

实际 E4 草稿构建在两题已选情况下自动续期会清空答案。`baseline-quiz-selection-lost.png`为真实续期后截图；真实HTTP及双页RED记录单列，旧截图不冒称最终构建。

## Prototype

修改页面前查看并保存当前两题已选截图`prototype-quiz-selected.png`，以此明确保留原控件和选中样式。主线程已明确授权最小Quiz watcher范围，不扩为全站设计。

## Reuse Decision

复用既有Play布局、题目卡片、单选项、按钮、结果和错误组件；同类型盲盒页状态及现有共享控件已检查。不新增视觉模式或设计例外。

## State Coverage

default/active：续期前后各两项选中，正常读题只一次；换会话清理由真实HTTP和双页验证。loading/error/disabled/empty/success：本批不新增样式分支，既有HTTP页面测试及原生提交/401浏览器验证另行记录；不将未截图的状态记作本轮视觉通过。hover/focus-visible/keyboard无新交互，复用现状；不宣称完整键盘验收。

## Viewport Coverage

360、768、1280、1920宽度，高900；四组覆盖中英文/明暗主题，实际浏览器检查无横向溢出或JS异常。无布局/字号改动，200%缩放未复核，reduced-motion启用reduce。

## Evidence

最终make build生成资源index SHA256为`8cf9218f92c17bf2e9850520ddf3f5ee3b99ae6e55c83dc10b3b6a67528b44a8`；四组捕获均exit0、safe=true，两项选择保留，Pinia与存储一致，已查看360和1280最终截图。各`updated-*`目录report.json记录实际index SHA256、选择数、Pinia/storage一致性与异常。只有报告safe=true且命令exit0才记为通过。基线、候选和最终构建均分开保存。

## Residual Risk

这份记录只证明隔离技术环境中的选择保留；不代表生产JWT、真实奖励发放或用户本机验收。未按本批标准验证的其他福利页仍待后续小步处理。
