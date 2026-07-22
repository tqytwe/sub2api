# 前端全链路体验与性能基线报告

> 状态：active
> 基线 commit：`3ce73505de3aa71a1d5c0da8cbb50296f6673d4e`
> 采集日期：2026-07-23
> 范围：首页、登录、钱包、支付、Play、管理员资金/订单/提现/战队/用户入口

## 结论摘要

当前最明显的首屏性能问题不是单个 Vue 页面，而是运行时注入的公开配置过大。生产 `index.html` 原始大小约 `435KB`，本地构建产物中的静态 `index.html` 只有 `1.3KB`。生产差值主要来自 `window.__APP_CONFIG__`，其中 `support_contact` 单项约 `426KB`，两个联系方式内联二维码分别约 `217KB` 和 `209KB`。

首屏还会在挂载后强制请求 `/api/v1/settings/public`，该接口原始大小约 `433KB`、Brotli 后约 `318KB`。因此首次进入页面会重复下载和解析两份大型公开配置。

## 采集方式

- 生产网络采样：Node HTTPS/curl，采样 3 次，记录状态码、TTFB、总耗时、响应体大小。
- 构建包体采样：`pnpm run build` 后统计 `backend/internal/web/dist/assets` 原始大小和 gzip 估算。
- 代码链路审计：检查 Vite 分包、路由懒加载、启动请求、公开配置注入路径。
- 未完成项：当前 worktree 没有项目内 Playwright 模块，`lighthouse` 未安装；本报告未包含真实浏览器 LCP/INP/TBT trace。下一阶段应补入自动浏览器性能脚本。

## 生产入口响应

| 路径 | 状态 | 原始响应体 | TTFB 中位数 | 总耗时中位数 | 说明 |
| --- | --- | ---: | ---: | ---: | --- |
| `/` | 200 | 434421 B | 329 ms | 493 ms | 首次样本较慢，后续稳定 |
| `/login` | 200 | 434421 B | 321 ms | 434 ms | 与首页同一 HTML |
| `/register` | 200 | 434421 B | 320 ms | 430 ms | 与首页同一 HTML |
| `/wallet` | 200 | 434421 B | 322 ms | 431 ms | 与首页同一 HTML |
| `/payment` | 200 | 434421 B | 321 ms | 430 ms | 与首页同一 HTML |
| `/play/arena` | 200 | 434421 B | 318 ms | 427 ms | 与首页同一 HTML |
| `/admin/users` | 200 | 434421 B | 322 ms | 431 ms | 与首页同一 HTML |
| `/admin/funds` | 200 | 434421 B | 320 ms | 428 ms | 与首页同一 HTML |
| `/admin/orders` | 200 | 434421 B | 318 ms | 427 ms | 与首页同一 HTML |
| `/admin/withdrawals` | 200 | 434421 B | 321 ms | 430 ms | 与首页同一 HTML |
| `/admin/play-ops` | 200 | 434421 B | 320 ms | 429 ms | 与首页同一 HTML |

Brotli 压缩后，生产 `/` 仍约 `318KB`。本地静态构建的 `index.html` 仅 `1387 B`，gzip 约 `700 B`。

## 生产 API 响应

| 接口 | 状态 | 原始响应体 | TTFB 中位数 | 总耗时中位数 | 说明 |
| --- | --- | ---: | ---: | ---: | --- |
| `/setup/status` | 200 | 58 B | 322 ms | 323 ms | 首屏挂载后会请求 |
| `/api/v1/settings/public` | 200 | 433103 B | 336 ms | 497 ms | 与 HTML 注入重复，最大问题 |
| `/api/v1/play/arena/daily/reward-summary` | 200 | 2384 B | 320 ms | 323 ms | 正常 |
| `/api/v1/user/wallet/summary` | 401 | 68 B | 288 ms | 288 ms | 未登录保护正常 |
| `/api/v1/admin/withdrawals` | 401 | 58 B | 290 ms | 290 ms | 未登录保护正常 |

## 公开配置体积拆解

生产 `window.__APP_CONFIG__` JSON 约 `432988 B`。最大字段如下：

| 字段 | 估算大小 | 说明 |
| --- | ---: | --- |
| `support_contact` | 426596 B | 两个联系方式内联二维码是主要来源 |
| `login_agreement_documents` | 4324 B | 四份协议 Markdown，可接受但可延迟 |
| `custom_menu_items` | 137 B | 小 |

`support_contact.contacts` 明细：

| 联系方式 | 类型 | 估算大小 |
| --- | --- | ---: |
| `legacy-contact` | `wechat` | 217127 B |
| `qq-1784660116626-2` | `qq` | 209337 B |

## 构建包体排行

| 资源 | 原始大小 | gzip 估算 |
| --- | ---: | ---: |
| `en-UHtremDC.js` | 690728 B | 204936 B |
| `AccountsView-CohL1Yhv.js` | 659362 B | 135202 B |
| `zh-Y06dg-xz.js` | 645626 B | 209975 B |
| `vendor-ui-C_jZSrVm.js` | 430775 B | 142315 B |
| `SettingsView-7zELH8RV.js` | 384294 B | 77846 B |
| `vendor-misc-D9K-xljd.js` | 322385 B | 112205 B |
| `index-zMvN063l.css` | 263872 B | 36169 B |
| `OpsDashboard-DBVyiim3.js` | 213848 B | 45820 B |
| `index-BJBfh79e.js` | 196192 B | 57448 B |
| `vendor-chart-DLGrz6L2.js` | 178340 B | 61250 B |
| `DocsView-D5lFFGae.js` | 177092 B | 53049 B |

全量 JS 原始约 `6.28 MiB`，全部静态资源约 `6.74 MiB`。

## 首屏资源估算

按当前构建和常见中文环境估算，不含生产注入 HTML：

| 页面 | 关键资源原始大小 | gzip 估算 |
| --- | ---: | ---: |
| 游客首页 `/` | 1695.2 KB | 502.6 KB |
| 登录 `/login` | 1582.7 KB | 474.2 KB |
| 钱包 `/wallet` | 1690.4 KB | 498.9 KB |
| 管理员用户 `/admin/users` | 2830.0 KB | 789.8 KB |

生产环境还要额外考虑 `index.html` 的 `318KB` Brotli 配置注入，以及挂载后的 `/api/v1/settings/public` 再次 `318KB` Brotli。

## 启动链路观察

- `main.ts` 在挂载前等待 `initI18n()` 和 `router.isReady()`，这让语言包和初始路由 chunk 直接进入首屏关键路径。
- `App.vue` 挂载后请求 `/setup/status`，再 `await appStore.fetchPublicSettings(true)`；虽然不阻塞 mount，但会造成大配置重复下载和解析。
- `router.beforeEach` 首次导航会 `authStore.checkAuth()`；登录态下会异步 `refreshUser()`。
- 登录态下 `App.vue` 监听认证状态，会预加载订阅并拉公告；这些适合后台化、去重和延迟。
- `i18n/index.ts` 的 `setLocale()` 会动态导入 router、app/auth/adminSettings store，用于更新标题；语言切换时可能额外触发较多模块。

## 主要问题分级

### P0：公开配置过大且重复下发

`support_contact.qr_image` 允许 data URL，并在公开配置和 HTML 注入中完整下发。当前两个二维码让每个页面 HTML 膨胀到 435KB，并让 `/api/v1/settings/public` 也达到 433KB。

建议：

- 新增轻量启动配置 DTO，只注入首屏必需字段：站点名、Logo、API base、语言/开关、OAuth/支付/Play 菜单开关、表格默认值、少量菜单项。
- `support_contact.contacts[].qr_image` 不进入 HTML 注入；公开设置接口默认返回 `qr_image_url` 或缩略信息，完整二维码改为按需接口获取。
- `login_agreement_documents.content_md` 默认不进首屏，只返回标题、ID、更新时间；登录/注册点击协议时再按需拉正文。
- `App.vue` 首屏不再强制等待或立即拉全量 public settings；使用轻量注入先渲染，再后台刷新小配置。
- 更新 `PublicSettingsInjectionPayload` schema 测试，改为“关键开关不得缺失，大字段禁止注入”的保护。

### P0：首屏公共包偏重

首屏必需包包含 `vendor-misc`、`vendor-i18n`、`vendor-vue`、`index`、语言包和全局 CSS。`vendor-misc` 原始 `322KB`，`index.css` 原始 `264KB`。

建议：

- 把 `dompurify`、`marked`、`driver.js`、`qrcode` 等从公共 vendor 中拆出到使用页面或组件所在 chunk。
- 首页 `HomeView` 中的 Markdown 清洗能力按需懒加载，不进入普通公共路径。
- 支付二维码、TOTP 二维码、公告 Markdown、Legal Markdown 等改成组件级动态 import。
- 检查全局 CSS，把仅首页、Play、Image Studio、管理后台使用的样式继续页面化。

### P1：语言包过大

`zh` 与 `en` 都超过 600KB 原始大小，且当前语言包在首屏初始化中是必需资源。

建议：

- 按域拆分语言包：`common/auth/public/user/admin/play/payment/image`。
- 首屏只加载 `common + 当前路由命名空间`。
- 建立 i18n key 对称测试，确保拆分后中文环境不泄漏英文。

### P1：管理员重页面需要二次拆分

`AccountsView` 原始 `659KB`，`SettingsView` 原始 `384KB`，`OpsDashboard` 原始 `214KB`。管理员页面还会引入 `vendor-ui` 和图表库。

建议：

- `AccountsView` 中导入、批量编辑、统计弹窗、OAuth 工具、模型白名单选择器独立异步组件。
- `SettingsView` 按 tab 拆分，首次只加载当前 tab。
- 图表只在图表区域出现时动态加载 `vendor-chart`。
- 大表格列设置、导出、批量操作按需加载。

### P1：启动请求可后台化

`/setup/status` 和 `/settings/public` 都在启动期出现。生产已安装状态下，`setup/status` 可缓存短时间或只在 `/setup` 路由强检查。

建议：

- 普通路由不再每次挂载检查 setup，改为失败兜底或只在 setup 相关路径检查。
- public settings 刷新使用 `requestIdleCallback` 或首屏后延迟。
- 登录态的订阅/公告预加载不阻塞路由切换，失败不刷屏。

### P2：浏览器指标体系缺失

当前仓库没有固定的性能基线脚本，无法在 CI 中防止包体和首屏退化。

建议：

- 增加 `scripts/perf-baseline.mjs`：采集 HTML/API/资源体积、关键 route chunk、资源预算。
- 后续接入 Playwright trace 或 Lighthouse CI，采集 FCP/LCP/TBT/INP 近似值。
- CI 增加预算：生产 HTML 注入配置上限、语言包上限、公共 vendor 上限、管理员单页 chunk 上限。

## 建议实施顺序

1. P0-A：轻量化公开配置和客服二维码按需加载。
2. P0-B：移除首屏重复 `/settings/public` 大请求，后台刷新轻量配置。
3. P0-C：拆 `vendor-misc` 中的 Markdown、二维码、引导库。
4. P1-A：i18n 领域拆包。
5. P1-B：管理员重页面 tab/弹窗异步拆分。
6. P2：引入性能预算脚本和浏览器性能脚本。

## 验收标准

- 生产 `index.html` Brotli 后应从约 `318KB` 降到 `50KB` 以下。
- `/api/v1/settings/public` 默认响应 Brotli 后应从约 `318KB` 降到 `80KB` 以下。
- 中文登录页首屏关键资源 gzip 估算降到 `300KB` 以内，不含浏览器缓存。
- 钱包普通用户首屏关键资源 gzip 估算降到 `350KB` 以内。
- 管理员用户页首屏关键资源 gzip 估算降到 `550KB` 以内。
- 中文/英文、浅色/深色、游客/用户/管理员本地浏览器验收无文案缺失、无布局溢出。
