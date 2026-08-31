# 极速蹬公开路由与 SEO 策略

> 状态：active
> 实现基线：`origin/play/main@ab69d89a` 上的 0.1.182 Fork 增量
> 最后更新：2026-08-31

本文规定极速蹬网页路由的可抓取边界。它不代表任何搜索引擎已收录、排名或验证；
站长平台的所有权和抓取结果必须由持有相应账号权限的人员单独核验。

## 路由归属

| 路由类别 | 路径 | 规则 |
| --- | --- | --- |
| 中文公开内容 | `/`、`/catalog`、`/catalog/:family`、`/docs`、`/about`、`/contact`、`/download/android` | `index,follow`，生成 canonical、中文默认 `x-default`、双语 hreflang 和真实可见内容对应的 JSON-LD。 |
| 英文公开内容 | `/en/`、`/en/catalog`、`/en/catalog/:family`、`/en/docs`、`/en/about`、`/en/contact` | `index,follow`，canonical 固定在英语路径，`x-default` 回到对应中文默认路径。 |
| 公开运行状态 | `/status`、`/en/status` | 可直接访问和站内链接，但为运行界面，不进 sitemap，必须 `noindex,nofollow`。 |
| 私有或认证页面 | 控制台、订阅、渠道监控、管理、创作空间、支付、Key、账号和设置等 | 页面 HTML 同时输出 `meta[name=robots]=noindex,nofollow` 与 `X-Robots-Tag: noindex, nofollow`，不得保留 canonical、hreflang 或路由 JSON-LD。 |
| API | `/api/*`、`/v1/*`、`/v1beta/*`、`/backend-api/*` 等 | 不作为网页内容进入 sitemap；robots.txt 只表达爬虫提示，鉴权和 API 路由保护仍由服务端负责。 |

## 模型目录与 API 兼容

- `/catalog` 与 `/en/catalog` 是浏览器模型目录、站内导航、canonical、hreflang、sitemap 和 `llms*.txt` 的唯一规范地址。
- 根 `GET /models` 是 OpenAI 兼容的受保护 API。带 `Authorization`、`x-api-key` 或
  `x-goog-api-key` 的请求，以及 JSON API 请求，继续保持 API 语义。
- 未带 API 凭据的浏览器文档导航请求 `/models` 返回 `308` 到 `/catalog`，保留查询参数。
  该响应和 API 兼容响应必须有 `Vary: Accept, Authorization, X-API-Key, X-Goog-API-Key`，
  使共享缓存按所有影响 HTML/API 分流的 API Key 头隔离变体。
- `/pricing` 和 `/model-plaza` 只作为浏览器兼容入口跳转到 `/catalog`；它们不产生独立
  sitemap、canonical 或 hreflang。

## 结构化内容边界

- 结构化数据只描述页面实际可见的 `Organization`、`WebApplication`、`ItemList` 或 FAQ
  内容；不为控制台数据、账号权益、渠道、价格私有态或运维状态虚构标记。
- 公开页面的嵌入 HTML 在 `#app` 内提供与当前路由标题、说明、目录和文档动作一致的语义化
  静态正文。该正文只生成给 `index,follow` 路由，Vue 挂载时会清空 `#app` 后渲染既有交互页；
  私有和 `noindex,nofollow` 路由不输出该兜底内容。
- `x-default` 始终为中文默认 URL，例如 `/`、`/catalog`、`/catalog/deepseek`、`/docs`、
  `/about` 和 `/contact`，不指向 `/en/*`。
- sitemap 仅列出当前可访问的公开内容，排除 `/home`、`/models`、创作空间和状态页。

## 跨层契约与 AI 路由清单

- `frontend/src/utils/public-route-seo-contract.json` 是公开 SEO 路由的可解析审查契约。
  它覆盖每条可索引路由的 title、description、Twitter title/description、canonical、hreflang
  和实际输出的 JSON-LD `@type` 集合。前端 `routeSeo.spec.ts` 在 hydration 后逐项校验，后端
  `embed_test.go` 对 SSR 注入结果逐项校验；改动任一 SEO map 时必须在同一 PR 更新契约并让两侧
  测试同时通过。
- `llms-full.txt` 的公开路由库存必须与 `promptSitemapStaticPaths` 保持一致，包含 DeepSeek、
  Qwen、Kimi、GLM 四个模型系列的中英文目录，以及文档、关于、联系和 Android 下载页；不得
  回填已废弃的 `/models` 路径或状态、账户、管理页面。

## 内容与外部核验

中文内容围绕大模型 API、DeepSeek API、Claude API 中转、低价 API、开发者接入和故障
排查；英文内容围绕具体模型、SDK、OpenAI-compatible integration、pricing 与
troubleshooting。内容分发渠道如微信搜一搜、知乎、掘金、CSDN、B 站、抖音和小红书属于
内容与品牌搜索，不等同于站内 SEO 收录。

上线后，由具有对应账号权限的运营人员依次核验百度、360、搜狗、神马，以及 Google、Bing
和 IndexNow 的所有权、sitemap、抓取、覆盖、canonical、hreflang 和 Core Web Vitals。
当前工程记录不声称这些外部核验已经完成。
