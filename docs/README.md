# 项目文档索引

> 状态：active
> 最后核验：2026-07-20
> 维护规则：当前实现以代码和测试为准；未登记在本索引中的 `docs/` 文档不得作为项目依据。

## 状态定义

| 状态 | 含义 |
|------|------|
| `active` | 当前行为或当前操作流程，功能变化时必须同步更新 |
| `reference` | 上游能力或外部对接参考，不代表极速蹬生产流程 |
| `historical` | 只用于追溯历史决策，不得作为当前实现依据 |
| `superseded` | 已被其他文档替代，只在归档索引中保留说明 |

## 当前文档

| 文档 | 状态 | 用途 | 维护责任 | 最后核验 |
|------|------|------|----------|----------|
| [服务器开发与生产验收流程](./DELIVERY_WORKFLOW.md) | `active` | 隔离开发、TDD、双审查、部署和本地浏览器验收 | 发布负责人 | 2026-07-16 |
| [Fork 定制登记](./FORK_CUSTOMIZATIONS.md) | `active` | 极速蹬定制的唯一权威登记表 | Fork 维护者 | 2026-07-20 |
| [上游同步手册](./UPSTREAM_SYNC_PLAYBOOK.md) | `active` | 合并、验证、部署和回滚 | 发布负责人 | 2026-07-20 |
| [图像工作室](./IMAGE_STUDIO.md) | `active` | 当前产品行为、接口和运维不变量 | 图像工作室维护者 | 2026-07-18 |
| [GPT / Grok 图片生成 API](./IMAGE_GENERATION_API.md) | `active` | 同步生成/编辑、`n=1-10`、实际尺寸和私有临时 URL | API 维护者 | 2026-07-18 |
| [Batch Image 持久批任务](./BATCH_IMAGE_API.md) | `active` | Gemini 多 prompt 预检、提交、恢复、结算和下载 | API 维护者 | 2026-07-18 |
| [异步图片任务](./ASYNC_IMAGE_TASKS.md) | `active` | 单请求异步队列、轮询、恢复边界和结果存储 | API 维护者 | 2026-07-20 |
| [Growth / Play](./GROWTH_PLAY.md) | `active` | 当前功能、开关、路由和短期 backlog | Growth 维护者 | 2026-07-15 |
| [增长埋点](./growth-analytics.md) | `active` | 当前埋点和指标口径 | Growth 维护者 | 2026-07-15 |
| [模型与价格](./MODEL_PRICING_CN.md) | `active` | 模型目录、参考价和实际计费关系 | 计费维护者 | 2026-07-15 |
| [余额账本治理](./BALANCE_LEDGER_ROLLOUT.md) | `active` | 统一资金流水、回填和剩余直写迁移边界 | 计费维护者 | 2026-07-20 |
| [Play、钱包与提现交付检查点](./PLAY_WALLET_WITHDRAWAL_ROLLOUT.md) | `active` | 分阶段门禁、验证证据、部署和本地浏览器验收状态 | Growth 与计费维护者 | 2026-07-20 |
| [批量图像 MVP](./BATCH_IMAGE_MVP.md) | `reference` | Batch provider、内部生命周期与运维参考 | API 维护者 | 2026-07-18 |
| [支付配置（中文）](./PAYMENT_CN.md) | `reference` | 上游内置支付配置 | 支付维护者 | 2026-07-15 |
| [Payment configuration](./PAYMENT.md) | `reference` | Upstream payment configuration | 支付维护者 | 2026-07-15 |
| [外部支付 Admin API](./ADMIN_PAYMENT_INTEGRATION_API.md) | `reference` | 外部支付系统对接 | 支付维护者 | 2026-07-15 |
| [管理员合规说明（中文）](./legal/admin-compliance.zh.md) | `active` | 管理员合规内容 | 合规维护者 | 2026-07-15 |
| [Admin compliance](./legal/admin-compliance.en.md) | `active` | English admin compliance content | 合规维护者 | 2026-07-15 |

## 仓库入口与上游参考

| 文档 | 状态 | 说明 |
|------|------|------|
| [开发指南](../DEV_GUIDE.md) | `active` | 本 Fork 的日常开发入口 |
| [README 中文](../README_CN.md) | `reference` | 上游项目总体说明，不替代本索引和同步手册 |
| [README English](../README.md) | `reference` | Upstream project overview |
| [README 日本語](../README_JA.md) | `reference` | Upstream project overview |
| [通用部署说明](../deploy/README.md) | `reference` | 上游部署方式；极速蹬生产仅使用 Zeabur |

## 历史归档

| 文档 | 状态 | 替代文档 |
|------|------|----------|
| [2026-07 增长世界 PRD](./archive/2026-07-growth-world-prd.md) | `historical` | [Growth / Play](./GROWTH_PLAY.md)、[图像工作室](./IMAGE_STUDIO.md) |
| [2026-07 Play 路线图](./archive/2026-07-growth-play-roadmap.md) | `superseded` | [Growth / Play](./GROWTH_PLAY.md) |

已完成且重复的 `IMAGE_STUDIO_COMPLETION_PLAN.md`、`MODEL_PRICING_PLAN.md` 已删除；需要追溯时使用 Git 历史。

## 维护要求

1. 新增 `docs/` 文档时必须在本索引登记状态、用途和维护责任。
2. 新增极速蹬特有行为时，必须更新 [Fork 定制登记](./FORK_CUSTOMIZATIONS.md) 和至少一项自动保护。
3. 方案类文档未上线前使用 `proposal` 标识，不得写入已生效的定制登记条目。
4. 每次上游同步后更新受影响文档的核验日期和上游 commit。
5. 交付流程以 [服务器开发与生产验收流程](./DELIVERY_WORKFLOW.md) 为准；旧的“仅服务器线上验收”表述不得继续作为完成标准。
