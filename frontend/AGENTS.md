# 极速蹬前端设计开发约束

本文件适用于 `frontend/` 下的全部文件。开始任何页面、组件、样式、图标或
可见交互改动前，必须先完整读取：

- `../docs/FRONTEND_DESIGN_SYSTEM.md`
- 根目录 `AGENTS.md`

## 强制工作流

1. 先查看目标页面当前实现，并查看至少一个同类型页面和现有共享组件。
2. 涉及视觉结果时，先记录当前画面：可使用已有截图，或在允许的开发环境中
   用 Playwright 截取基线；不得只读模板后凭想象修改。
3. 先复用现有布局、`Icon.vue`、按钮、表单、Dialog、Table、Toast 和状态组件。
4. 新视觉模式必须先补充设计规范或共享组件，不得直接在业务页面复制一套。
5. 修改后检查 default、hover、active、focus-visible、loading、disabled、
   empty、error、success；不适用的状态要在评审说明中明确。
6. 检查 360px、768px、1280px 和宽屏；视觉风险较高时增加 200% 缩放、
   浅色、深色、中英文和 reduced-motion。
7. 新增 `../docs/visual-reviews/YYYY-MM-DD-<slug>.md`，使用仓库模板记录实际
   基线、修改后画面、状态与视口证据；没有视觉证据记录时门禁必须失败。
   视觉产物必须声明 `artifact_mode`，真实 PNG 必须可解码；静态审查板不得冒充
   浏览器截图，且必须留下最终浏览器验收风险。
8. 运行 `pnpm design:check`、`pnpm lint:check`、`pnpm typecheck` 和相关测试。

## 硬性规则

- 功能图标统一使用 `src/components/icons/Icon.vue`；禁止在页面或普通组件内
  新增手写 SVG。品牌 Logo、供应商 Logo、图表和艺术场景可以例外。
- 图标按钮必须有可访问名称或 tooltip；同一工具栏图标使用一致尺寸和描边。
- 页面根节点不得自行增加任意像素 `max-width`、居中容器、页面 gutter、
  全屏背景或页面级滚动；这些职责属于共享 PageFrame/Layout。
- 登录后的控制台和管理后台页面默认必须铺满侧栏右侧可用宽度；只有文档、
  法务、短流程状态页和个人表单这类阅读/表单场景允许居中限宽。
- Route UI runtime 落地前，新路由必须在视觉记录中声明目标 contract，且不得新增
  私有页面宽度、居中、全屏高度或页面滚动；resolver 落地后再由类型/AST 强制完整
  shell、frame、密度、标题、滚动、背景和客服位置。
- 运营界面禁止新增 `rounded-2xl`、`rounded-3xl`、渐变按钮、彩色阴影和
  `transition-all`。卡片 8px，控件 6-8px，浮层 12px。
- 禁止在业务组件散落十六进制颜色；颜色必须来自语义 token。品牌、图表和
  明确艺术页面的特殊色值必须集中定义。
- 禁止取消焦点轮廓而不提供符合规范的 `focus-visible` 替代。
- Hover 不能承载唯一内容或操作；非交互卡片不得伪装成可点击元素。
- 卡片不得嵌套卡片；页面 section 不是浮动卡片。
- 不得为了“更活泼”任意增加缩放、漂浮、持续脉冲或布局位移。

## 例外

确实需要违反增量检查规则时，在对应文件加入：

```text
design-governance-allow: <rule-name> - <具体原因>
```

允许的 rule name 为 `inline-svg`、`transition-all`、`large-radius`、
`page-shell-ownership`、`raw-color`、`focus-reset`、`decorative-gradient`、
`continuous-motion`。
例外必须属于品牌、图表、艺术构图或第三方支付等真实边界，并在代码审查中单独确认。
