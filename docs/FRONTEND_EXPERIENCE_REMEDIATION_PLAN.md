# 极速蹬前端体验统一整改计划

> 状态：reviewed
> 范围：Sub2API 全部前端，以及与 NextChat managed 壳之间的共同体验合同

## 1. 目标

本计划解决的不是单个页面“再美化一点”，而是导致体验持续分裂的系统问题：

- 页面宽度、居中和 gutter 各自实现，宽屏大量留白或突然全宽。
- 图标、按钮、圆角、颜色、阴影和 hover 状态没有统一所有权。
- 页面只实现静态画面，缺少 loading、empty、error、disabled、success 和键盘状态。
- 客服、公告、导航、标题和错误恢复在不同页面各维护一份。
- NextChat managed 壳与 Sub2API 品牌体验不一致，但又不能破坏上游聊天核心。
- 改完没有视觉证据和自动门禁，后续开发会再次复制旧问题。

整改完成后的判断标准是：同一任务在不同页面具有同一视觉语言、同一数据来源、
同一错误恢复方式，并且新代码无法轻易绕过这些规则。

### 1.1 已查看的现状视觉基线

- 当前悬浮客服卡以纯文本拼接 QQ 和微信，窄宽度下账号、渠道名和复制按钮发生
  非语义断行；没有二维码层级，也没有“主联系方式/更多方式”的信息结构。
- 当前后台设置仍以一个“客服联系方式”文本输入框保存整段内容，无法独立配置
  类型、账号、链接、描述、启用状态、排序和双二维码。
- 这两处不是单纯换样式即可修复，必须由结构化配置、共享客服组件和统一页面框架
  一起解决；后续不得再把多个渠道压回一段自由文本。

## 2. 已确认的产品决定

- NextChat 只统一 managed 品牌壳、入口、客服、锁定/错误页和关键交互状态；
  不重写上游聊天消息、输入、滚动、持久化和会话核心。
- 公告图片使用平台托管对象存储和独立公告资产模型。
- 公告正文允许 Unicode emoji、语义色块和固定高亮；不允许任意颜色、内联 CSS
  或作者原始 HTML。
- 客服配置以 Sub2API 为唯一来源，登录页、用户页、悬浮入口和 NextChat 使用
  同一公开/登录投影。
- “实时更新”采用 revision 加 `no-store` 和轻量刷新，不引入 WebSocket。

## 3. P0 架构整改

### 3.1 页面框架

新增 Route UI contract 和统一 resolver。每个新路由必须声明 shell、frame、
density、heading、scroll、surface、mobile title、support 和 art variant。
Route host 统一生成 PageFrame/PageHeader；业务页面不再声明页面宽度、背景和滚动。

正式宽度只保留 `compact`、`reading`、`form`、`content`、`workspace` 和
`fluid`，具体像素值由设计系统统一定义。旧 `6xl/7xl/max-w-none` 进入迁移白名单，
不允许新用法。

### 3.2 单一数据源

- 导航：共享 navigation catalog，以 route name 解析路径。
- 客服：共享 `SupportContactPanel`，公开设置和 bootstrap 来自同一后端 builder。
- 公告：共享 `AnnouncementContent` 和同一 Markdown renderer。
- 标题：共享 title resolver，首页只显示站点名，内页显示本地化页面名加站点名。
- 认证回跳：唯一参数 `redirect`，共享安全解析器覆盖密码、邮箱验证、2FA 和 OAuth。

### 3.3 NextChat 状态机

session、bootstrap 和 group switch 使用独立 request id/AbortController。进入工作台
必须满足 `session=authenticated` 且 `bootstrap=ready`。明确处理：

- login-required、feature-disabled、token-expired、session-expired。
- account-locked、no-group、no-model、insufficient-balance。
- bootstrap-unavailable、timeout、network-error。

每个状态提供明确解释、重试、返回控制台、重新进入和客服入口；生产页隐藏组件栈，
只显示诊断 ID。失败不能无限 skeleton，也不能默认要求清理本地数据。

### 3.4 公告资产与内容安全

公告资产独立于 Image Studio。数据库保存稳定 `storage_key`、SHA-256、MIME、宽高、
大小和状态，不保存临时签名 URL。上传状态机为 uploading -> ready；删除先标记再
异步清理。公告保存事务从 Markdown AST 提取资产引用，拒绝双重 `asset_ids` 真相。

后端解析 Markdown AST，拒绝 raw HTML、非法 URL、外链/Data 图片、超量图片和危险
节点。历史公告增加 `content_format` 与 publication revision；修改内容后重新未读，
并用 expected version 防止多人覆盖。

图片只接受实际解码成功的静态 JPEG/PNG/WebP，校验尺寸、像素、体积和 EXIF
Orientation。Bell、Popup、详情和预览必须生成相同 DOM。

## 4. P1 视觉与交互整改

### 4.1 基础 token 和组件

- 建立 surface、text、border、action、status、focus 语义 token。
- 统一间距、圆角、阴影、排版和数字格式。
- 收敛 Icon、Button、Input、Select、Dialog、Toast、DataTable、EmptyState、
  Skeleton、PageFrame 和 PageHeader。
- 禁止业务页面手写功能 SVG、散落色值、渐变按钮、彩色阴影和 `transition-all`。

### 4.2 完整状态

每个异步区域覆盖 default/loading/empty/error/success/stale/no-permission。
按钮覆盖 hover/active/focus-visible/loading/disabled。错误必须保留输入、关联字段、
提供修复建议和重试；Toast 不能成为阻断错误的唯一载体。

### 4.3 可访问性

- 全部操作具备 2px、3:1 的 focus-visible。
- Dialog 只有顶层响应 Escape，背景 inert，焦点圈定并正确归还。
- 路由前进聚焦页面标题或 main，后退恢复合理滚动和焦点。
- 表单使用 `aria-invalid`、`aria-describedby` 和错误摘要。
- Skeleton 对辅助技术隐藏，加载区域使用 `aria-busy`。
- 虚拟表格宣布完整行数和真实行索引，或提供非虚拟访问模式。
- 所有拖拽提供键盘/点击替代和重置入口。

## 5. 信息架构与首页

首页首屏和顶部导航应清楚回答“这是什么、能做什么、下一步去哪”：

- 产品创作入口 `/ai`。
- 模型与价格。
- 使用文档/帮助中心。
- 公告与服务状态。
- 玩法福利或产品活动。
- 联系客服。
- 登录/注册或用户工作台。

旧 `/image-studio` 只保留深链兼容并标识旧版，不再作为默认创作入口。首页 CTA、
快捷操作、侧栏和提示词“用于创作”统一进入 `/ai`。游客完成认证后返回原目标。

## 6. 迁移顺序

1. 设计 token、Route UI contract、PageFrame 和自动门禁。当前先落地 fail-closed
   增量门禁与结构化视觉证据；Route host/runtime contract 是下一阶段实现，不冒充已完成。
2. Profile/Redeem 作为表单样板。
3. Dashboard/Wallet/Payment 作为内容页样板。
4. Announcements/AvailableChannels/Admin 表格作为密集布局样板。
5. 客服与公告共享组件，全站替换旧显示点。
6. Home 导航、CTA、标题和认证回跳。
7. Play/Image Studio 仅在统一基础上保留艺术表达。
8. NextChat managed 壳、状态机、客服和错误恢复。
9. 删除 legacy width/meta/component allowlist。

每个路由原子迁移，不能在同一路由同时保留旧 shell 和新 PageFrame。

## 7. 验收矩阵

- 宽度：360、390、599、600、601、639、640、641、768、1024、1280、1440、
  1600、1920、2560。
- 高度：568、667、800、900、1080、1440。
- 模式：浅色/深色、中文/英文、reduced-motion、200% zoom、键盘和触屏。
- 状态：default、loading、empty、error、disabled、selected、open、success、
  warning、stale、no-permission。
- 身份：游客、普通用户、管理员；NextChat 开启/关闭、会话过期和账户锁定。

硬性通过条件：无非业务横向滚动；gutter 与 frame 误差不超过 1px；文字和操作不
重叠；短屏操作可达；焦点不丢失；颜色不是唯一状态信号；图表有文本替代。

## 8. 门禁与证据

- `pnpm design:check` 检查新增功能 SVG、散落色值、大圆角、任意页面框架、
  焦点清除、装饰渐变和 `transition-all`。
- CI 的 lint、test 和 production build 前必须先执行 `design:verify`。Docker 内的纯
  production build 不依赖 Git diff 或治理文档，避免构建上下文缺少 `.git` 时误伤发布。
- 每个可见界面 PR 必须提交 `docs/visual-reviews/YYYY-MM-DD-<slug>.md`。
- 高风险页面使用 Playwright 对比桌面/移动、浅深色和关键状态，并执行重叠检查。
- 部署证明必须包含真实路由和新 bundle；最终视觉验收仍由本地浏览器完成。

## 9. 多角色评审结论

- 产品/信息架构：优先修复 `/ai` 入口网络、认证回跳、首页导航和错误恢复。
- 视觉系统：Route UI contract 必须强制，页面不能自行选择宽度和壳。
- 可访问性：焦点、Dialog 栈、表单错误、Toast、虚拟表格和 reduced-motion
  必须成为共享组件合同。
- 前端架构：Sub2API 与 NextChat 共享业务合同，不共享 Vue/React CSS 实现。
- 安全/存储：公告必须使用稳定托管资产、后端 AST 白名单、版本并发和可恢复清理。

以上结论均纳入本计划和设计规范；后续实现不得把它们降级为“可选优化”。
