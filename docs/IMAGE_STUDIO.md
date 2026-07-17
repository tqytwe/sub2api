# 图像工作室当前实现

> 状态：active
> 用户入口：`/image-studio`
> 生产验收：`https://www.jisudeng.com/image-studio`
> 最后核验：2026-07-17

## 产品行为

图像工作室是登录用户的持久创作工作台，不是 Batch Image 的包装层。桌面端采用设置栏与作品区双栏布局；移动端使用“创作 / 作品”标签，生成后切到作品区，并通过路由元信息隐藏客服浮窗。每个用户最多同时保有 2 个活动任务，页面可同时轮询两路任务并在刷新或服务重启后恢复。

表单顺序为模板、必填描述、规格、高级设置、费用与生成。首次进入选择目录中的第一个模板，回访恢复本地保存的上次模板。成功结果在作品区置顶，历史任务只展示一次；失败保留输入并允许按原规格继续创作。

当前模板：

| ID | 用途 | 默认规格 | 示例资源 |
|----|------|----------|----------|
| `ecom-white-bg` | 电商白底主图 | `1024x1024`，4 张 | `/image-studio/templates/ecom-white-bg.webp` |
| `xhs-cover` | 社交竖版封面 | `1024x1536`，4 张 | `/image-studio/templates/xhs-cover.webp` |
| `free-create` | 通用自由创作 | `1024x1024`，1 张 | `/image-studio/templates/free-create.webp` |

模板 API 返回中英文 `label`、`description`、`preview_url` 和默认值；`preview_emoji` 仅供旧客户端回退，服务端 Prompt 模板不通过 JSON 暴露。

## 规格与模型

新客户端只使用规范比例 `1:1`、`2:3`、`3:2`、`9:16`、`16:9`，分辨率档位为 `1K / 2K / 4K`。旧请求的 `3:4`、`4:3` 分别兼容解析为 `2:3`、`3:2`，但目录不再返回旧别名。尺寸反推按有序目录执行，不能依赖 Go map 遍历顺序。

API Key 决定可用分组和图像模型。模型来自网关映射并受分组自定义模型列表限制；只有一个选项时前端自动选择。模型 capability 决定可用尺寸和 quality，模型或 capability 异常时禁止生成并给出原因。

估价以实际 API Key 分组为上下文：模型定价为基础，图像分档价可覆盖基础价，再应用与网关后扣一致的用户专属分组倍率或独立图像倍率。Image Studio 当前只接受钱包计费的 standard 分组，不接受 subscription 分组。创建任务在同一 PostgreSQL 事务中预占估价余额；hold 同时形成每个 item 的最高收费快照，后续媒体价格调整不会让 capture 超过预占或使任务卡在 running。未开始或未确认 provider 成本的 item 不收费；provider 已返回并确认实际成本后，即使后续取消先提交或本地对象持久化重试耗尽，也按每 item hold 快照封顶结算，剩余金额自动 release。

编辑估价接收当前用户拥有的 ready reference IDs。服务端读取图片真实尺寸，以保守的 patch/token 上界和 fidelity reserve 计算每次输出的图片输入 hold，因此页面估价与任务预占使用同一算法；上游报告图片输入 token 时按权威明细结算，Grok 编辑未报告该明细时按同一真实尺寸保守 token 结算，不能免费释放输入 hold，多预占部分自动释放。

模型 capability 决定 create/edit、固定尺寸或比例分辨率、quality、background、输出格式、压缩、input fidelity、透明背景和参考图上限。`gpt-image-2-*` 版本模型继承 `gpt-image-2` profile；透明背景只允许 PNG/WebP，JPEG/JPG 组合在提交前拒绝。OpenAI 与 Grok 的 create/edit 路由分别固定，不按字符串猜测 provider。

## 接口

公开目录：

```text
GET /api/v1/image-studio/templates
GET /api/v1/image-studio/capabilities
```

JWT 鉴权接口：

```text
GET    /api/v1/image-studio/models
GET    /api/v1/image-studio/estimate
POST   /api/v1/image-studio/generate
POST   /api/v1/image-studio/references
DELETE /api/v1/image-studio/references/:id
GET    /api/v1/image-studio/jobs/active
GET    /api/v1/image-studio/jobs
GET    /api/v1/image-studio/jobs/:id
POST   /api/v1/image-studio/jobs/:id/cancel
DELETE /api/v1/image-studio/jobs/:id
GET    /api/v1/image-studio/jobs/:id/download
GET    /api/v1/image-studio/assets/:id/thumbnail
GET    /api/v1/image-studio/assets/:id/content
GET    /api/v1/image-studio/assets/:id/download
```

对外开发者直调的 GPT/Grok 图像 API 使用 API Key 鉴权，入口为 `https://api.jisudeng.com/v1/images/generations` 和 `https://api.jisudeng.com/v1/images/edits`，见 [GPT / Grok 图片生成 API](./IMAGE_GENERATION_API.md)。图像工作室只负责控制台模板、估价、异步 job 和图库，不替代对外 API 文档。

空白描述必须返回 `IMAGE_STUDIO_PROMPT_REQUIRED`，不得替换成默认 `product`。`POST /generate` 必须携带稳定 `Idempotency-Key`；同一提交发生网络失败时复用 key，新提交使用新 key。通用幂等响应缓存之外，job 表还有 `(user_id, idempotency_key_hash)` 唯一防线，防止“任务和余额已提交、HTTP 响应记录不确定”时重复创建。

任务和每张输出都持久化。worker 使用 lease、heartbeat 和确定性 item/asset ID；provider 已返回但资产尚未落库时先写加密请求之外的二进制 checkpoint，重启后只恢复持久化，不重复调用 provider。单 item 最多尝试 3 次，任务状态为 `pending / running / completed / partial / failed / cancelled`；取消与完成竞态由数据库事务裁决：checkpoint 先提交则允许该输出完成持久化并收费，取消先提交则拒绝新的 checkpoint 和资产但保留已确认 provider 成本用于结算。成功 item 写入的资产在 job 结算事务提交为 `completed/partial` 前不可查询、预览或下载。managed usage 持久化失败会保留实际成本并写 reconciliation，runner 使用幂等账务键自动重放。

OpenAI worker 可在没有 checkpoint 时按 item 尝试上限重试；Grok 同步图片请求没有已确认的幂等重放语义，因此第二次 claim 若仍无 durable checkpoint 会直接失败，不再次调用 provider。两类 provider 已有 checkpoint 时都只恢复资产持久化。

编辑模式先通过 reference 接口上传 1 至 4 张私有图片，再提交 reference IDs。上传仅接受真实可解码的 PNG/JPEG/WebP，单张最多 20 MiB；每用户最多保留 20 个、合计 80 MiB 的未过期引用。上传额度由 PostgreSQL advisory lock 和 lease 跨实例共享：每用户最多 2 个并发上传、每分钟最多 20 次，异常实例遗留 slot 最迟 10 分钟过期。接受 edit job 时会复制为 job-owned references，因此临时上传过期或被删除不会破坏已接受任务。

## 隐私、资产与清理

- 数据库只保存最终组合 Prompt 的 SHA-256，不保存明文 Prompt；API 响应不暴露 `prompt_hash`。
- 引用图、原图和缩略图写入配置的私有 data volume 下 `image-studio/<user-id>/`，目录和文件权限分别为 `0700`、`0600`；数据库只保存相对 `storage_key`、类型、字节数和真实宽高。
- 预览、下载、查询和删除都以当前 JWT 用户校验 job/asset 所有权。
- 默认任务和资产保留 7 天；用户关闭自动清理时 `retain_days=0`，任务不设置过期时间。
- 图库按 12 条 terminal 历史任务分页，活动任务由 `/jobs/active` 独立返回，不占历史页名额；回访用户首屏从第一页选择最新的 completed/partial 作品，但不会触发成功埋点、任务奖励或清理草稿。历史资产没有缩略图时回退鉴权 `/content`，不会循环请求不存在的 thumbnail。单任务 ZIP 最多 100 个资产、64 MiB、2 路外链抓取和 30 秒超时，外链失败会返回错误而不是静默漏文件。
- content、thumbnail、download 和 ZIP 使用一致的私有缓存策略。
- `PlayGrowthRunner` 每 5 分钟清理过期上传/任务、重放账务 reconciliation 和对象删除 outbox。过期任务清理会在同一事务锁定任务、为引用图/原图/缩略图写 deletion outbox 并删除 metadata；对象删除失败时 outbox 保留并异步重试。手动删除 job 使用同一 outbox 约束，避免对象删除失败形成永久 orphan。
- 文件系统 reconciliation 只扫描超过 1 小时宽限期的对象，并用真实 PostgreSQL metadata 同时核对临时 reference、job-owned reference、原图和缩略图；查询失败时保留全部对象，只有四类 metadata 都未引用的对象才会删除。
- 任务失败或资产写入失败必须落为失败状态，不能留下显示完成但图库为空的任务。

## 运维与验收

功能由 `image_studio_enabled` 控制。当前对象存储后端是实例私有文件系统，因此生产必须使用覆盖应用 data directory 的持久卷，并保持所有副本可访问同一目录；没有共享卷时必须单副本运行。上游合并后运行 `./scripts/check-fork-integrity.sh`，再在 Zeabur 生产环境验证：模板预览、必填校验、Key/模型自动选择、比例与档位、估价、余额不足、双任务与刷新/重启恢复、部分成功、取消、参考图编辑、高级设置、缩略图、分页、ZIP、鉴权预览、删除和暗色主题。

迁移 runner 会为一次完整迁移运行固定一条 PostgreSQL session，在该 session 上获取 advisory lock、执行所有事务与 `_notx.sql` 阶段并校验释放锁结果。迁移 `192_image_studio_persistent_jobs.sql` 与 `194_image_studio_asset_derivatives.sql` 按可恢复阶段执行：每条语句使用独立短事务和 `lock_timeout`，长 backfill/constraint validation 不携带前置 `ALTER TABLE` 强锁；全部阶段成功后才记录 `schema_migrations`。对应索引继续在 `_notx.sql` 中使用 `CONCURRENTLY`，失败重试只删除 PostgreSQL 标记为 invalid 的索引。

GPT/Grok 多张和多 prompt 批量调用见 [多张 / 批量生图调用](./BATCH_IMAGE_API.md)。内部异步批量任务保留在 [Batch Image MVP](./BATCH_IMAGE_MVP.md)，不得把两套任务或计费链路合并。
