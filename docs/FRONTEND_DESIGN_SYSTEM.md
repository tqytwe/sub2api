# 极速蹬前端设计系统与视觉开发规范

> 状态：active
> 生效日期：2026-07-21
> 适用范围：Sub2API 公共页、认证页、用户控制台、管理员后台及共享组件

本文档是极速蹬前端视觉与交互实现的权威规范。需求稿、历史页面或局部组件与
本文冲突时，新改动以本文为准；旧页面通过计划内迁移逐步收敛，不以旧代码的
不一致为新实现依据。

## 1. 开发前视觉检查

任何可见改动都必须完成以下步骤：

1. 打开并阅读目标页面、同类型页面和相关共享组件。
2. 查看当前实际画面或已有截图，记录宽度、密度、层级和已有交互状态。
3. 确认能否复用现有布局、图标、按钮、表单、表格、浮层和状态组件。
4. 明确页面属于运营界面还是允许艺术表达的页面。
5. 写出本次会影响的 default、hover、active、focus、loading、disabled、
   empty、error、success 状态。

视觉改动完成后必须保留可复核的前后对比。截图不是最终生产验收，但属于前端
实现和代码审查证据。

每次可见界面改动必须新增
`docs/visual-reviews/YYYY-MM-DD-<slug>.md`。记录必须包含目标路由、基线画面、
实际复用的共享组件、状态矩阵、视口、前后截图或录像位置，以及未解决风险。
模板占位符、只写“已检查”或只有代码链接不算有效证据。

## 2. 视觉原则

- 清晰优先：信息层级、任务路径和可扫描性高于装饰。
- 一致优先：复用成熟模式，不为单一页面创造新的颜色、圆角或交互语言。
- 状态完整：所有异步和可交互元素都必须有完整状态，不只实现静态画面。
- 内容适配：运营后台保持安静、紧凑、可重复操作；Home、Play、Image Studio
  可以有表达性，但仍服从语义状态和可访问性规则。
- 响应式由内容决定：布局根据内容容器变化，不根据某台设备或截图硬编码。

## 3. 页面框架

页面宽度使用语义档位，不允许业务页面直接选择任意数字：

| 档位 | 最大宽度 | 用途 |
| --- | ---: | --- |
| compact | 672px | 回调、结果、短错误和单任务流程 |
| reading | 800px | 文档、法律文本和公告详情 |
| form | 960px | Profile、Redeem 和复杂表单 |
| content | 1152px | Dashboard、Wallet、Payment 和默认页面 |
| workspace | 1600px | 表格、运维、Play 和 Image Studio |
| fluid | 无固定上限 | Home、NextChat 和真正全屏工作区 |

页面 gutter 固定为：

- 360-479px：16px
- 480-767px：20px
- 768-1199px：24px
- 1200px 以上：32px

页面根节点不得自行承担宽度、居中、背景、全屏高度或滚动。共享 Layout 和
PageFrame 负责页面框架，页面只负责内容结构。

### Route UI contract（迁移目标）

以下 contract 是页面框架整改的固定目标。当前仓库尚未完成 resolver、PageFrame
和全路由迁移，因此机器门禁现阶段先阻止 route view 新增私有宽度、居中、全屏高度
和页面滚动；完成 P0 基础设施后，再把“新路由必须声明 contract”升级为类型和 AST
硬门禁。

```ts
type RouteUIContract = {
  shell: "app" | "public" | "bare" | "fullscreen";
  frame: "compact" | "reading" | "form" | "content" | "workspace" | "fluid";
  density: "comfortable" | "compact";
  headingMode: "chrome" | "content" | "hidden";
  scrollMode: "document" | "contained" | "workspace";
  surfaceMode: "page" | "subtle" | "art";
  mobileTitle: "header" | "content" | "hidden";
  supportPlacement: "auto" | "hidden" | "inline";
  artVariant: "none" | "home" | "play" | "image-studio";
};
```

- Contract 只表达页面意图，不引用 Vue 组件名、Tailwind class 或 CSS selector。
- Route host 负责生成 PageFrame、PageHeader、背景、gutter 和滚动容器。
- 业务页面根节点禁止 `max-w-*`、`mx-auto`、`min-h-screen`、`h-screen` 和
  页面级 `overflow-y-auto`。
- `fluid` 仅用于 Home、NextChat、画布和真正全屏工作区，不能成为逃生档位。
- 迁移期旧 meta 由 resolver 兼容；resolver 落地后，新路由不得继续新增旧布尔开关。

## 4. 间距、圆角和层级

- 间距只使用：4、8、12、16、20、24、32、40、48px。
- 普通控件高度 40px，移动触控目标至少 44px。
- 紧凑表格行高至少 44px，普通表格行高至少 52px。
- 运营卡片圆角 8px；普通控件 6px；大控件 8px；popover/modal 12px。
- 全圆角只用于头像、状态点、徽章和真正的 pill。
- 默认卡片使用边框，最多使用 elevation-1；popover 使用 elevation-2；
  modal 使用 elevation-3。
- 运营界面禁止彩色阴影、装饰渐变、悬浮位移和卡片嵌套卡片。

## 5. 排版和数据

- 页面标题：24px / 32px / 600。
- 区块标题：18px / 26px / 600。
- 卡片标题：16px / 24px / 600。
- 正文：14px / 22px。
- 密集数据：13px / 20px。
- 辅助文字：12px / 18px。
- 10px 仅允许艺术页面中的非关键信息。
- 字间距固定为 0；字号不得跟随 viewport 连续缩放。
- 金额、Token、百分比、时长和统计数字使用 `tabular-nums` 与 `Intl`。
- 图表必须有 tooltip、空/错状态，并提供表格或文本数值替代。

## 6. 颜色

业务组件只使用语义 token：

- surface：page、panel、raised、sunken、overlay。
- text：primary、secondary、muted、inverse。
- border：subtle、default、strong、focus。
- action：primary、secondary、ghost、danger。
- status：info、success、warning、danger、neutral。

状态不能只依赖颜色；必须同时使用图标、文字、形状或位置。正文对比度至少
4.5:1，大文本和非文本状态至少 3:1。艺术页可以定义 accent，但不得覆盖状态色、
正文灰阶、焦点色和表单错误色。

## 7. 图标

- 功能图标唯一入口为 `src/components/icons/Icon.vue`。
- 默认采用 24x24 viewBox、`currentColor`、圆角端点、1.5 描边。
- 尺寸固定为 12、16、20、24、32px；同一工具栏不得混用视觉重量。
- 熟悉动作优先使用图标按钮，例如关闭、复制、下载、刷新、删除、前进和返回。
- 不熟悉图标必须提供 tooltip；所有图标按钮必须有 `aria-label` 或可见名称。
- 禁止用 emoji 代替功能图标，禁止在业务模板手写一份相似但不一致的 SVG。
- 品牌、支付渠道、模型供应商、图表和艺术插图不受功能图标限制，但必须集中管理。

图标名称使用动作或对象语义，例如 `copy`、`refresh`、`support`，禁止按某个页面
命名。新增图标前必须搜索现有目录；相同语义只保留一个图标，不允许同义图标因
线宽、尺寸或圆角不同而并存。

## 8. 交互状态

| 状态 | 统一行为 |
| --- | --- |
| hover | 仅增强颜色、边框或阴影，不改变布局；只在支持 hover 的设备启用 |
| active | 80-120ms 明确反馈，不改变控件尺寸 |
| focus-visible | 至少 2px、3:1 对比度，不被 overflow 裁切 |
| loading | 保持按钮宽度和名称，防重复提交，区域设置 `aria-busy` |
| disabled | 使用原生 disabled 或正确的 aria-disabled，并说明不可用原因 |
| success | 重要结果就地保留，并通过 status/polite 宣布 |
| error | 保留输入，显示原因、解决方法、重试，并关联到具体字段 |
| empty | 区分初始空、筛选空、无权限和请求失败，提供对应下一步 |

禁止只写 `transition-all`。每个组件只声明实际变化的属性。非交互内容不得通过
hover 阴影、缩放或鼠标手势暗示可点击。

## 9. 浮层、表格和表单

- Dialog、popover、drawer 和预览统一使用共享浮层基础设施。
- 只有顶层浮层响应 Escape；背景 inert；焦点圈定；关闭后回到触发点。
- 图标工具栏、Select、日期选择器和菜单必须支持完整键盘操作。
- DataTable 排序使用按钮和 `aria-sort`；行只有在整体可点击时才有 hover。
- 表单错误使用 `aria-invalid`、`aria-describedby` 和可聚焦错误摘要。
- Toast 成功/信息使用 polite，阻断错误使用 assertive；有操作时不自动消失。
- Skeleton 对辅助技术隐藏，加载区域负责宣布忙碌状态。

## 10. 响应式与动画

- 最低检查视口为 360、768、1280px；宽布局增加 1600、1920、2560px。
- 页面不得产生非业务需要的横向滚动。
- 200% 缩放时文本、按钮、浮层和表格操作仍可到达。
- 中英文和长模型名不得重叠、裁切关键操作或改变固定控件尺寸。
- `prefers-reduced-motion: reduce` 下取消位移、缩放、旋转、闪烁、shimmer、
  ping 和持续脉冲，只保留不超过 100ms 的必要透明度反馈。

## 11. 公告内容

- 公告继续保存 Markdown，不保存编辑器生成 HTML。
- 图片只能通过平台托管的公告资产上传进入内容；禁止外链图片、Data URL、SVG
  和任意对象存储临时签名 URL。
- 允许 Unicode emoji，但不把 emoji 当成功能图标。
- 文字强调只开放固定高亮和语义块：
  `==高亮==`、`::info[]`、`::success[]`、`::warning[]`、`::danger[]`、
  `::muted[]`，以及 `[!INFO]`、`[!SUCCESS]`、`[!WARNING]`、`[!DANGER]`、
  `[!NOTE]` 块引用提示。
- 禁止任意文字颜色、渐变、内联 CSS、作者原始 HTML、`class` 和 `style`。
- Bell、Popup、公告详情和编辑预览必须使用同一 Markdown renderer、白名单和
  `AnnouncementContent` 组件，不能各自解析一遍。
- 图片上传、引用、发布、删除和清理使用平台托管公告资产服务，底层复用现有对象
  存储；不能复用短期、私有的 Image Studio 用户资产，也不能保存外部图片热链。

## 12. 导航、品牌和标题

- 首页主导航至少覆盖产品能力、模型与价格、文档/帮助、创作入口、查账入口和客服；
  入口是否显示由统一导航 catalog 和公开配置决定。
- 导航项使用稳定 route name，由 router 解析 URL；页面和布局不得各写一份路径。
- 浏览器首页标题固定为站点名，例如 `极速蹬`，不得显示原始路由 token `HOME`。
- 内页标题格式为 `{本地化页面名} | {站点名}`；站点名来自公开配置。
- 登录、注册、邮箱验证、2FA 和 OAuth 使用同一安全 `redirect` 合同，认证后返回
  原目标；禁止页面自行发明 `return`、`next` 等平行参数。

## 13. 艺术页面边界

Home、Play、Image Studio 可以覆盖 composition、media、display type 和 accent。
它们不能覆盖：

- 功能图标体系。
- 语义状态颜色。
- 表单、按钮、焦点和错误行为。
- 浮层、Toast、Dialog 和表格行为。
- 正文可读性、数据格式、触控尺寸和 reduced-motion。

艺术例外必须在文件中使用 `design-governance-allow` 说明原因。

## 14. 组件所有权与变更规则

- `PageFrame/PageHeader`：页面宽度、gutter、标题、滚动和 surface。
- `Icon.vue`：全部功能图标。
- `Button/Input/TextArea/Select`：控件尺寸、状态、错误和焦点。
- `BaseDialog/Popover/Toast`：浮层、层级、焦点圈定和消息宣布。
- `DataTable`：表格密度、排序、空错状态和虚拟化语义。
- `SupportContactPanel`：所有客服信息展示。
- `AnnouncementContent`：所有公告内容渲染。

新增视觉模式必须先改变共享组件或本文档，再迁移业务页面。设计系统规则变更要在
同一 PR 更新文档、门禁和视觉审查记录；不得在单一页面悄悄创造“特殊版”。

## 15. 完成门禁

当前已自动强制的是：治理文档存在、Git 基线可解析、增量反模式检查、真实视觉记录
与修改前后产物。Route UI runtime、Playwright/axe CI 和组件状态自动探测仍按整改计划
分阶段落地，不得在交付说明中写成已经完成。

每个视觉任务必须确认：

- 已读取本规范和 `frontend/AGENTS.md`。
- 已查看当前画面、同类页面和共享组件。
- 未新增平行图标、按钮、卡片、表单或弹窗体系。
- 页面宽度、间距、圆角、颜色和排版符合规范。
- 所有适用交互状态完整。
- 键盘、触屏、浅深色、中英文和响应式风险已检查。
- `pnpm design:check`、lint、typecheck、相关测试和 build 均通过。
- 视觉风险较高时已完成 Playwright 前后截图和重叠检查。
- 已新增并填写 `docs/visual-reviews/YYYY-MM-DD-<slug>.md`。
