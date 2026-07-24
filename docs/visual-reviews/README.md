# 视觉审查记录

本目录保存所有可见界面改动的本地审查证据。它的目的不是堆截图，而是保证开发者
和代理在修改前真正看过当前画面，并能说明为什么复用或扩展某个共享模式。

## 使用规则

1. 从 `TEMPLATE.md` 新建 `YYYY-MM-DD-<slug>.md`，填写顶部
   `visual-review-manifest` JSON；每个可见改动文件都必须出现在 `changed_files`。
2. 记录目标路由、身份、主题、语言和基线截图。
3. 写明复用的 PageFrame、图标、控件、浮层和状态组件。
4. 覆盖本次适用的交互状态和视口。
5. 每次前端可见改动必须先提交原型设计图片，并在 manifest 的
   `prototype_artifacts` 中引用；口头描述、代码截图、外部链接、空白图和 1x1
   占位图都不能替代原型图片。
6. 将原型图、修改前后截图、录像或可复核产物保存到
   `docs/visual-reviews/assets/<slug>/`；无效路径、空文件和外部临时路径不能通过门禁。
7. `artifact_mode` 必须声明为 `browser-capture` 或 `static-review-board`。静态审查板只能作为无浏览器环境下的辅助证据，必须在 residual risk 中写明仍需浏览器截图或最终验收；不得把静态图冒充浏览器截图。
8. PNG 产物必须是真实可解码图片，门禁会校验 PNG chunk、CRC、像素数据和最小尺寸，1x1 或伪造 header 不能通过。
9. 不得提交模板占位符、凭据、Cookie、用户隐私或生产密钥。

`pnpm design:check` 会验证结构化字段、文件覆盖、至少两个视口、键盘与
reduced-motion 结论、产物来源模式，以及原型图、修改前后产物确实存在且可解码。
纯标题或空壳记录不能通过。
