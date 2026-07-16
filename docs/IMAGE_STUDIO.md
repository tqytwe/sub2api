# 图像工作室当前实现

> 状态：active
> 用户入口：`/image-studio`
> 生产验收：`https://www.jisudeng.com/image-studio`
> 最后核验：2026-07-15

## 产品行为

图像工作室是登录用户的单次创作工作台，不是 Batch Image 的包装层。桌面端采用设置栏与作品区双栏布局；移动端使用“创作 / 作品”标签，生成后切到作品区，并通过路由元信息隐藏客服浮窗。

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

估价以实际 API Key 分组为上下文：模型定价为基础，图像分档价可覆盖基础价，再应用分组或独立图像倍率。余额不足在创建任务前拒绝。

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
GET    /api/v1/image-studio/jobs/active
GET    /api/v1/image-studio/jobs
GET    /api/v1/image-studio/jobs/:id
DELETE /api/v1/image-studio/jobs/:id
GET    /api/v1/image-studio/assets/:id/content
GET    /api/v1/image-studio/assets/:id/download
```

对外开发者直调的 GPT/Grok 图像 API 使用 API Key 鉴权，入口为 `https://api.jisudeng.com/v1/images/generations` 和 `https://api.jisudeng.com/v1/images/edits`，见 [GPT / Grok 图片生成 API](./IMAGE_GENERATION_API.md)。图像工作室只负责控制台模板、估价、异步 job 和图库，不替代对外 API 文档。

空白描述必须返回 `IMAGE_STUDIO_PROMPT_REQUIRED`，不得替换成默认 `product`。生成接口先建立 `pending` 任务，再在脱离请求取消的后台上下文执行网关调用；客户端通过 job 接口轮询 `pending / running / completed / failed`。

## 隐私、资产与清理

- 数据库只保存最终组合 Prompt 的 SHA-256，不保存明文 Prompt；API 响应不暴露 `prompt_hash`。
- 生成资产写入配置的 data volume 下 `image-studio/<user-id>/`，数据库保存相对 `storage_key`、类型和字节数。
- 预览、下载、查询和删除都以当前 JWT 用户校验 job/asset 所有权。
- 默认任务和资产保留 7 天；用户关闭自动清理时 `retain_days=0`，任务不设置过期时间。
- `PlayGrowthRunner` 每 5 分钟清理过期任务，并同步删除本地资产。
- 任务失败或资产写入失败必须落为失败状态，不能留下显示完成但图库为空的任务。

## 运维与验收

功能由 `image_studio_enabled` 控制，持久卷必须覆盖应用的 data directory。上游合并后运行 `./scripts/check-fork-integrity.sh`，再在 Zeabur 生产环境验证：模板预览、必填校验、Key/模型自动选择、比例与档位、估价、余额不足、异步恢复、失败重试、鉴权预览、下载、删除和暗色主题。

GPT/Grok 多张和多 prompt 批量调用见 [多张 / 批量生图调用](./BATCH_IMAGE_API.md)。内部异步批量任务保留在 [Batch Image MVP](./BATCH_IMAGE_MVP.md)，不得把两套任务或计费链路合并。
