# 极速蹬 Fork 定制登记

> 状态：active
> 当前验证基线：`upstream/main@be6cd1250c10ba2812305dcf071da133e4d738f6`
> 对应合并提交：`3a23bd68be50654f165acfde7e537f51c1ec5abf`
> 最后核验：2026-07-16

本文档是 `play/main` 相对上游的定制权威登记表。只有已经落地的行为进入受保护条目；视频工作室等未实现方案只能作为 `proposal` 独立保存，不能登记成已上线能力。

## 登记摘要

| ID | 领域 | 状态 | 自动保护 |
|----|------|------|----------|
| `FORK-BRAND-001` | 品牌、首页、登录布局和主题 | active | branding 脚本 + 前端测试 |
| `FORK-NAV-002` | 用户侧栏和 Growth 导航 | active | integrity 脚本 + AppSidebar 测试 |
| `FORK-PLAY-003` | Growth / Play 系统 | active | integrity 脚本 + Go/Vitest 测试 |
| `FORK-IMAGE-004` | 图像工作室 | active | integrity 脚本 + Go/Vitest 测试 |
| `FORK-PRICING-005` | 模型目录和价格优先级 | active | integrity 脚本 + Go 测试 |
| `FORK-DEPLOY-006` | 分支与 Zeabur 部署 | active | integrity 脚本 |
| `FORK-OAUTH-007` | OAuth Cookie 域共享 | active | Go 测试 |
| `FORK-PUBLIC-008` | 公共页面与可见性 | active | integrity 脚本 + Go 测试 |
| `FORK-MIGRATION-009` | 自定义数据库迁移 | active | 完整文件名清单检查 |
| `FORK-BILLING-010` | 计费归属与充值联动 | active | integrity 脚本 + Go 测试 |

所有条目的上游冲突都必须逐段审查，禁止对整个文件直接使用 `ours` 或 `theirs`。

## FORK-BRAND-001 品牌、首页、登录布局和主题

- 产品目的：公开首页、认证页和控制台保持极速蹬 ink 黑白品牌，不回退到上游 teal 视觉。
- 不变量：`AuthLayout` 保留 `auth-page`、`asideMode` 和定制 CSS；首页保留极速蹬内容与资源；首页页脚和自定义首页保留 LMSpeed Provider 2039 的 claim badge，`frontend/index.html` 保留无 JavaScript 备用标记；Tailwind `primary` 为 ink；版本号只对管理员展示。
- 关键位置：`frontend/src/views/HomeView.vue`、`frontend/src/components/home/LmspeedBadge.vue`、`frontend/index.html`、`frontend/src/components/layout/AuthLayout.vue`、`frontend/src/styles/home-view.css`、`frontend/src/styles/auth-layout-jisudeng.css`、`frontend/tailwind.config.js`。
- 冲突策略：吸收上游可访问性和业务修复，视觉结构、品牌资源及色板保留 Fork 语义。
- 验证：`scripts/check-jisudeng-branding.sh`、`frontend/src/components/layout/__tests__/AppSidebar.spec.ts`；线上检查首页、登录、注册、浅色和深色主题。

## FORK-NAV-002 用户侧栏和 Growth 导航

- 产品目的：普通用户直接看到“模型与价格”、图像工具和“玩法福利”，不暴露渠道运维入口。
- 不变量：用户侧栏包含 `/models`、`/image-studio`、`/batch-image` 和 `/growth-group`；Growth 子项由功能开关过滤；普通用户侧栏不得出现 `/available-channels` 或 `/monitor`；管理员渠道监控保留在管理区。
- 关键位置：`frontend/src/components/layout/AppSidebar.vue`、`frontend/src/utils/featureFlags.ts`、`frontend/src/router/index.ts`。
- 冲突策略：上游新增导航项先判断面向用户还是管理员，再合入对应分组，不能恢复上游默认用户渠道入口。
- 验证：integrity 脚本和 AppSidebar 测试；线上分别使用普通用户与管理员账号检查。

## FORK-PLAY-003 Growth / Play 系统

- 产品目的：以签到、Arena、盲盒、答题、Agent Team、任务、活动和 Hub 提升激活与留存。
- 不变量：`/api/v1/play/*`、`/api/v1/public/models`、`/play`、`/check-in`、`/arena`、`/blindbox`、`/quiz-quest`、`/agent-team` 路由存在；功能按 `play_*` 设置 fail-closed；Hub 聚合待办、余额、活动和图像工作室状态。
- 关键位置：`backend/internal/server/routes/play.go`、`backend/internal/service/setting_play_runtime.go`、`backend/internal/service/play_hub.go`、`frontend/src/views/user/PlayHubView.vue`、`frontend/src/content/play-features.ts`。
- 冲突策略：保留上游共享支付、用户与用量修复；Play 表、路由、设置和奖励账本不得被移除或绕过。
- 验证：Play service tests、`public_growth_teaser_test.go`、相关前端 utils tests；线上逐项检查开关关闭和开启状态。

## FORK-IMAGE-004 图像工作室

- 产品目的：登录用户在控制台内完成模板选图、描述、规格、估价、异步生成、预览和下载。
- 不变量：描述必填且后端返回 `IMAGE_STUDIO_PROMPT_REQUIRED`；不保存明文 Prompt，只保存 SHA-256；任务异步执行并可恢复轮询；资产优先写入 `data/image-studio/`，预览和下载校验资产所属用户；默认保留 7 天，可关闭自动清理；移动端隐藏客服浮窗。
- 接口与数据：`/api/v1/image-studio/*`；表 `image_studio_jobs`、`image_studio_assets`；设置 `image_studio_enabled`。
- 关键位置：`backend/internal/server/routes/image_studio.go`、`backend/internal/service/image_studio*.go`、`frontend/src/views/user/ImageStudioView.vue`、`frontend/src/composables/useImageStudioWorkspace.ts`。
- 冲突策略：网关和计费可吸收上游修复；Prompt 隐私、任务所有权、本地持久化、模板字段、规范比例和工作台 UI 不得回退。
- 验证：image studio Go tests 和前端 gallery/size/workspace tests；线上完成生成、刷新恢复、预览、下载、删除和失败重试。

## FORK-PRICING-005 模型目录和价格优先级

- 产品目的：区分外部官方参考价、本站参考价和真实扣费价，并让模型绑定明确的业务分组。
- 不变量：官方价只用于对比；本站价优先于 legacy catalog fallback；渠道价仍是已配置渠道的实际基础；分组或用户倍率生成实付价；`group_ids=NULL` 才允许按平台兼容匹配，非空数组只能进入指定分组；刷新官方价不得覆盖手工本站价或渠道价。
- 接口与数据：`site_model_catalog`、`group_ids`、`official_*`、`GET /public/model-pricing`、登录价目和 Admin model catalog APIs。
- 关键位置：`backend/internal/service/model_catalog*`、`backend/internal/service/model_pricing_resolver.go`、`backend/internal/repository/model_catalog_repo.go`、`frontend/src/views/public/ModelsView.vue`、`frontend/src/views/admin/ModelCatalogView.vue`。
- 冲突策略：上游模型能力可合入，但不得将公开参考价重新接入扣费，也不得用 platform 猜测覆盖显式分组绑定。
- 验证：`model_catalog_service_test.go`、`model_pricing_resolver_test.go`、`model_pricing_sync_test.go` 及真实调用抽样对账。

## FORK-DEPLOY-006 分支与 Zeabur 部署

- 产品目的：极速蹬在服务器完成可复现开发与验证，从 `origin/play/main` 触发 Zeabur 构建，并由用户本地电脑完成生产验收。
- 不变量：服务器使用隔离 Git worktree，业务行为执行 TDD 和逐任务规格/代码质量审查；测试、完整构建和 Fork integrity 全通过后才提交并先推送审查分支；完整 GitHub CI 只在目标为 `play/main` 的 PR 上执行一次，普通分支 push 和生产 push 不重复完整测试；确认后以非 rebase、非强推方式进入 `play/main`；只有 `origin/play/main` 触发 Zeabur；部署 commit 和健康状态确认后，由用户本地电脑浏览器访问 `https://www.jisudeng.com/`，以游客、普通用户和管理员三种身份验收；禁止用本地服务、localhost 或服务器浏览器作为最终验收结论。
- 关键位置：`AGENTS.md`、[服务器开发与生产验收流程](./DELIVERY_WORKFLOW.md)、`scripts/push-github-and-deploy.sh`、`deploy/zeabur.template.yaml`、`.cursor/rules/sub2api-server-only-verify.mdc`、[上游同步手册](./UPSTREAM_SYNC_PLAYBOOK.md)。
- 冲突策略：上游 Docker 文档只作为参考，不能改变极速蹬生产分支和验收入口。
- 验证：服务器完整闸门、PR 单次 GitHub CI、Zeabur 部署 commit/健康状态，以及用户本地电脑的游客、普通用户、管理员生产浏览器验收；按风险补充浅色/深色和 API/数据库对账。

## FORK-OAUTH-007 OAuth Cookie 域共享

- 产品目的：`jisudeng.com`、`www.jisudeng.com` 和 API 子域之间的 LinuxDo OAuth 流程共享临时状态 Cookie。
- 不变量：极速蹬域名返回 `.jisudeng.com`；localhost、IP 和未知域名使用 host-only Cookie；安全、HttpOnly 和 SameSite 约束继续沿用认证实现。
- 关键位置：`backend/internal/handler/auth_linuxdo_oauth.go`、`backend/internal/handler/auth_linuxdo_oauth_test.go`。
- 冲突策略：可以吸收上游 OAuth 安全修复，但必须保留域共享函数及测试。
- 验证：`TestOAuthCookieDomain`，线上分别从 apex 与 `www` 发起/完成 OAuth。

## FORK-PUBLIC-008 公共页面与可见性

- 产品目的：游客可浏览极速蹬首页、文档、模型价格和 Play 展示，登录用户看到与账号分组匹配的内容。
- 不变量：`/home`、`/models`、`/docs` 和公开 Play 页面存在；`public_models_enabled` 控制游客目录；`available_channels_enabled` 控制登录价目数据而不是恢复用户侧栏“可用渠道”入口；公开接口不泄露渠道密钥、账号或内部定价配置。
- 关键位置：`frontend/src/views/public/`、`frontend/src/content/public-docs-data.zh.ts`、`backend/internal/service/public_model_catalog.go`、`backend/internal/service/setting_public.go`。
- 冲突策略：上游公共设置字段变更需要合并到 DTO 和前端类型，但极速蹬可见性语义优先。
- 验证：public model/catalog、public settings 和 teaser tests；游客、登录用户、管理员三种身份线上检查。

## FORK-MIGRATION-009 自定义数据库迁移

- 产品目的：保留 Play、品牌默认值、图像工作室和模型目录的数据库结构与数据修复。
- 不变量：下列文件名完整存在且已应用文件不可改写；上游出现同数字前缀时允许并存，不能按编号覆盖。
- 冲突策略：新增迁移使用新的完整文件名；禁止修改已部署 SQL 的内容来解决冲突。
- 验证：integrity 脚本逐文件检查，部署后检查 `schema_migrations`。

```text
170_play_foundation.sql
171_play_extended.sql
172_play_retention.sql
173_play_vip.sql
174_play_team_affiliate.sql
175_play_campaigns.sql
176_play_campaign_name_i18n.sql
177_marketing_fixes.sql
177_play_quiz_i18n_and_pool.sql
178_phase1_growth_world.sql
179_play_sidebar_defaults.sql
180_site_subtitle_jisudeng.sql
181_jisudeng_public_model_pricing.sql
182_image_studio_asset_storage.sql
183_model_catalog.sql
184_image_studio_asset_url_nullable.sql
185_model_catalog_official_prices.sql
186_model_catalog_billing_lookup.sql
186_model_sync_jobs_repair.sql
187_model_catalog_group_scope.sql
189_restore_growth_rollback_defaults.sql
```

## FORK-BILLING-010 计费归属与充值联动

- 产品目的：防止 API Key、订阅或批量任务被错误归属到其他用户，同时让成功充值触发可选 Play boost。
- 不变量：扣费前验证 API Key 与用户归属；订阅扣费验证订阅所有者；余额冻结同样验证归属；粘性会话种子按 API Key 隔离；模型目录参考价不能覆盖真实渠道计费；支付订单完成后再授予 recharge boost，boost 失败不得回滚已完成充值。
- 关键位置：`backend/internal/repository/usage_billing_repo.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/gateway_service.go`、`backend/internal/service/payment_fulfillment.go`、`backend/internal/service/play_recharge_boost.go`。
- 冲突策略：上游支付状态机和安全修复必须合入；归属校验、真实计费优先级与充值后 Play 联动必须保留。
- 验证：usage billing unit/integration tests、session hash tests、model pricing tests、payment lifecycle tests；线上以测试订单检查余额到账和 boost 状态。

## 更新规则

1. 新增或改变 Fork 行为时，先更新对应条目；没有对应条目时创建新的稳定 ID。
2. 条目必须至少绑定一项静态检查或行为测试；高风险计费、认证和权限行为必须有测试。
3. 上游同步完成并验证后，统一更新本文顶部基线及受影响文档日期。
4. 删除定制必须在同一 PR 删除登记、保护检查和专属测试，不能只删代码。
