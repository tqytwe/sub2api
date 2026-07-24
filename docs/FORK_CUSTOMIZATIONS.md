# 极速蹬 Fork 定制登记

> 状态：active
> 当前验证基线：`upstream/main@e625ce3b3b3b955b7c3afc93221f7c5f0ae55aa8`
> 对应合并提交：待本同步 PR 合入 `play/main` 后回填
> 最后核验：2026-07-23

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
| `FORK-IMAGE-011` | Images API、Gateway async 与 Batch 运行时 | active | integrity 脚本 + Go/Vitest/集成测试 |
| `FORK-UI-012` | 前端设计系统与视觉治理 | active | design governance 脚本 + lint/typecheck/视觉检查 |
| `FORK-RISK-013` | IP 风险检测与批量注册发现 | active | integrity 脚本 + Go/PostgreSQL 集成测试 |
| `FORK-ADMIN-014` | 管理员用户批量处置 | active | Go/Vitest/API contract 测试 |

所有条目的上游冲突都必须逐段审查，禁止对整个文件直接使用 `ours` 或 `theirs`。

## FORK-BRAND-001 品牌、首页、登录布局和主题

- 产品目的：公开首页、认证页和控制台保持极速蹬 ink 黑白品牌，不回退到上游 teal 视觉。
- 不变量：`AuthLayout` 保留 `auth-page`、`asideMode` 和定制 CSS；首页保留极速蹬内容与资源；首页页脚和自定义首页保留 LMSpeed Provider 2039 的 claim badge，`frontend/index.html` 保留无 JavaScript 备用标记；Tailwind `primary` 为 ink；版本号只对管理员展示。
- 关键位置：`frontend/src/views/HomeView.vue`、`frontend/src/components/home/LmspeedBadge.vue`、`frontend/index.html`、`frontend/src/components/layout/AuthLayout.vue`、`frontend/src/styles/home-view.css`、`frontend/src/styles/auth-layout-jisudeng.css`、`frontend/tailwind.config.js`。
- 冲突策略：吸收上游可访问性和业务修复，视觉结构、品牌资源及色板保留 Fork 语义。
- 验证：`scripts/check-jisudeng-branding.sh`、`frontend/src/components/layout/__tests__/AppSidebar.spec.ts`；线上检查首页、登录、注册、浅色和深色主题。

## FORK-UI-012 前端设计系统与视觉治理

- 产品目的：让公共页、认证页、用户控制台和管理员后台使用稳定的页面框架、功能图标、视觉 token、圆角、间距、交互状态与可访问性规则，防止新增页面继续复制平行组件和局部风格。
- 不变量：任何 `frontend/` 可见改动必须先读取 `frontend/AGENTS.md` 和 [前端设计系统与视觉开发规范](./FRONTEND_DESIGN_SYSTEM.md)，查看当前画面和同类型实现，先提交原型设计图片并明确改动边界，优先复用共享组件；功能图标统一走 `Icon.vue`；业务页面不得新增任意页面宽度、手写功能 SVG、散落色值、`transition-all`、超大圆角或无替代的焦点清除。艺术页和品牌场景例外必须带具体原因并接受单独审查。
- 关键位置：`frontend/AGENTS.md`、`docs/FRONTEND_DESIGN_SYSTEM.md`、`docs/FRONTEND_EXPERIENCE_REMEDIATION_PLAN.md`、`docs/frontend-design-governance.json`、`docs/visual-reviews/`、`frontend/src/components/icons/Icon.vue`、`scripts/check-frontend-design-governance.mjs`、`scripts/check-frontend-design-governance.test.mjs`、`frontend/package.json`。
- 冲突策略：吸收上游业务和可访问性修复，但新视觉实现必须映射到极速蹬语义规则；不得以“上游原样”或“历史页面已有”为理由继续扩散不一致。
- 验证：`cd frontend && pnpm design:check`、`pnpm lint:check`、`pnpm typecheck`、相关 Vitest、production build，以及 `docs/visual-reviews/` 中可复核的原型设计图片、Playwright 前后截图、浅深色、中英文、响应式和 reduced-motion 记录。

## FORK-NAV-002 用户侧栏和 Growth 导航

- 产品目的：普通用户直接看到“模型与价格”、图像工具和“玩法福利”，不暴露渠道运维入口。
- 不变量：用户侧栏包含 `/models`、`/image-studio`、`/batch-image` 和 `/growth-group`；Growth 子项由功能开关过滤；普通用户侧栏不得出现 `/available-channels` 或 `/monitor`；管理员渠道监控保留在管理区。
- 关键位置：`frontend/src/components/layout/AppSidebar.vue`、`frontend/src/utils/featureFlags.ts`、`frontend/src/router/index.ts`。
- 冲突策略：上游新增导航项先判断面向用户还是管理员，再合入对应分组，不能恢复上游默认用户渠道入口。
- 验证：integrity 脚本和 AppSidebar 测试；线上分别使用普通用户与管理员账号检查。

## FORK-PLAY-003 Growth / Play 系统

- 产品目的：以签到、Arena、盲盒、答题、Agent Team、任务、活动和 Hub 提升激活与留存，并为管理员提供受控、可审计的战队成员关系修复能力。
- 不变量：`/api/v1/play/*`、`/api/v1/public/models`、`/play`、`/check-in`、`/arena`、`/blindbox`、`/quiz-quest`、`/agent-team` 路由存在；功能按 `play_*` 设置 fail-closed；Hub 聚合待办、余额、活动和图像工作室状态。管理员战队修复必须通过 `/api/v1/admin/play/teams/:id/member-candidates` 预检、`/members` 幂等写入和 `/events` 追踪；移动或显式历史生效时间必须使用 JWT 管理员 TOTP step-up，管理员 API Key 禁止；写入在同一事务锁定用户、来源/目标战队和成员关系，并与结算快照共用战队锁；移动的旧 `left_at` 与新 `joined_at` 完全一致；源/目标事件和审计动作不得保存邀请码或令牌。Token 农场日榜公开汇总必须从已结算日榜 period 和 `play_reward_ledger` 读取，最近发放不暴露邮箱，历史缺失的 rank/token 只能通过 period 时间窗安全回补，不能改写旧奖励流水；当前预估不能把昨日已结算榜标为今日排名。中文和英文运行时资源必须保持 key 对称，中文 Play Ops 和 Token 农场不显示英文操作状态或标签。
- 关键位置：`backend/internal/server/routes/play.go`、`backend/internal/server/routes/admin.go`、`backend/internal/service/setting_play_runtime.go`、`backend/internal/service/play_hub.go`、`backend/internal/service/play_admin_team_repair.go`、`backend/internal/repository/play_repo_admin_team_repair.go`、`frontend/src/views/user/PlayHubView.vue`、`frontend/src/views/admin/PlayOpsView.vue`、`frontend/src/content/play-features.ts`。
- 冲突策略：保留上游共享支付、用户与用量修复；Play 表、路由、设置和奖励账本不得被移除或绕过。
- 验证：Play service tests、`play_admin_team_repair_test.go`、真实 PostgreSQL `play_repo_team_integration_test.go`、`play_handler_team_repair_test.go`、`play_admin_routes_test.go`、`PlayOpsView.spec.ts`、`admin.play.teamRepair.spec.ts`、`adminPlayOpsParity.spec.ts`、`public_growth_teaser_test.go` 和相关前端 utils tests；线上逐项检查开关关闭和开启状态，并由本地管理员浏览器完成中英文、浅色和深色战队修复验收。

## FORK-IMAGE-004 图像工作室

- 产品目的：登录用户在控制台内完成模板选图、描述、规格、参考图编辑、估价、持久异步生成、预览、分页和下载。
- 不变量：描述必填且后端返回 `IMAGE_STUDIO_PROMPT_REQUIRED`；生成要求稳定 `Idempotency-Key`；不保存明文 Prompt，只保存 SHA-256 和加密 worker payload；每用户最多 2 个活动任务，支持 partial/cancelled、lease/heartbeat/restart recovery；余额按用户专属有效倍率和参考图输入成本先 hold，已确认 provider 成本的实际收费不超过创建时的每 item hold 快照，再 capture/release；Grok 编辑缺少权威图片输入 token 时按真实参考图尺寸保守结算；引用图和资产写入私有持久卷，所有读取按用户鉴权；任务清理先在同一事务写对象删除 outbox 并删除 metadata，对象失败由 outbox 重试；默认保留 7 天，可关闭自动清理；移动端隐藏客服浮窗。
- 接口与数据：`/api/v1/image-studio/*`；表 `image_studio_jobs`、`image_studio_items`、`image_studio_assets`、`image_studio_references`、`image_studio_job_references`、`image_studio_billing_reconciliations`、`image_studio_object_deletions`、`image_studio_upload_slots`；设置 `image_studio_enabled`。
- 关键位置：`backend/internal/server/routes/image_studio.go`、`backend/internal/service/image_studio*.go`、`frontend/src/views/user/ImageStudioView.vue`、`frontend/src/composables/useImageStudioWorkspace.ts`。
- 冲突策略：网关和计费可吸收上游修复；Prompt 隐私、幂等、任务所有权、持久 job/item、checkpoint、余额预占、引用图私有化、capability 路由、模板字段、规范比例和工作台 UI 不得回退。
- 验证：image studio Go unit/integration tests、真实 PostgreSQL recovery/billing/reference/outbox tests 和前端 gallery/size/workspace tests；线上完成双任务生成、刷新与重启恢复、部分成功、取消、编辑、高级设置、缩略图、分页、ZIP、删除和失败重试。

## FORK-IMAGE-011 Images API、Gateway async 与 Batch 运行时

- 产品目的：为开发者提供行为可预测的同步 Images、单请求异步 Gateway 和多 prompt 持久 Batch，并让管理员直接看到三套图片运行时是否真正就绪。
- 不变量：JSON 接受可选 UTF-8 BOM，空 prompt 在账号选择和上游调用前返回 `400 IMAGE_PROMPT_REQUIRED`；同步 `n=1..10`，不兼容多图字段的通道拆成多个 `n=1` 子请求并按实际成功图片结算，流式只允许 `n=1`；`response_format=url` 必须写入私有结果存储并返回同 API Key 鉴权临时 URL，流式 partial 只返回 Base64 预览、completed 返回临时 URL，存储不可用时返回 `503 IMAGE_RESULT_STORAGE_UNAVAILABLE`；响应公开请求模型、上游模型、请求尺寸和解码后的实际尺寸，不拉伸或伪造 usage。
- Gateway async 不变量：加密请求信封进入 Redis ready/active 队列，由有界 worker 执行；固定加密密钥、owner 范围幂等、queued/processing/terminal Redis CAS、lease/lock 心跳、租约丢失取消上游、终态清除请求信封；worker 使用 task ID 作为稳定请求/计费 ID，订阅上下文恢复失败必须 fail-closed，不能回退余额计费；重启只恢复 queued，无法证明安全的 processing 明确失败。
- Batch 不变量：`BATCH_IMAGE_ENABLED=true` 只有在 PostgreSQL、Redis、队列和 worker 同时就绪时才接受新提交；允许 API 关闭但 queue 继续排空和结算；列表在关闭时仍可读，models 预检区分全局关闭、运行时异常、分组、账号、模型和价格；`Idempotency-Key` 可选，提供时按 owner 唯一并检测请求冲突，提交返回 `202`、`Location`、`Retry-After`；单 item 最多 4、单任务最多 200，5/10 张使用独立 items；Vertex 开关关闭时不得进入 provider registry。
- 运行与文档：`GET /api/v1/admin/ops/image-runtimes/health` 返回 Gateway async、Batch、Image Studio 的存储、数据库、Redis、worker、backlog、最老任务和最近错误；公开 `/docs`、Batch 控制台说明、维护文档和首页 CTA 必须与真实路由及认证边界一致。
- 关键位置：`backend/internal/service/openai_images*.go`、`backend/internal/service/image_task*.go`、`backend/internal/repository/image_task_*.go`、`backend/internal/service/batch_image*.go`、`backend/internal/service/image_runtimes_health.go`、`frontend/src/content/public-docs-data.zh.ts`、`frontend/src/views/user/BatchImageGuideView.vue`。
- 冲突策略：可吸收上游 Images 和队列实现改进，但不得恢复 data URL 伪装、无界 goroutine、半开启运行时、非原子终态、错误订阅计费或文档与生产能力不一致。
- 验证：同步 Images、URL 所有权/过期、流式 URL、Gateway async CAS/lease/recovery、Batch readiness/provider/idempotency 的单元与真实 PostgreSQL/Redis 集成测试，公开文档/首页/Batch 前端测试；生产按 queue 先开、API 后开的两阶段流程提交真实 5 张和 10 张 Gemini Batch。

## FORK-PRICING-005 模型目录和价格优先级

- 产品目的：区分外部官方参考价、本站参考价和真实扣费价，并让模型绑定明确的业务分组。
- 不变量：官方价只用于对比；本站价优先于 legacy catalog fallback；渠道价仍是已配置渠道的实际基础；分组或用户倍率生成实付价；`group_ids=NULL` 才允许按平台兼容匹配，非空数组只能进入指定分组；刷新官方价不得覆盖手工本站价或渠道价。
- 接口与数据：`site_model_catalog`、`group_ids`、`official_*`、`GET /public/model-pricing`、登录价目和 Admin model catalog APIs。
- 关键位置：`backend/internal/service/model_catalog*`、`backend/internal/service/model_pricing_resolver.go`、`backend/internal/repository/model_catalog_repo.go`、`frontend/src/views/public/ModelsView.vue`、`frontend/src/views/admin/ModelCatalogView.vue`。
- 冲突策略：上游模型能力可合入，但不得将公开参考价重新接入扣费，也不得用 platform 猜测覆盖显式分组绑定。
- 验证：`model_catalog_service_test.go`、`model_pricing_resolver_test.go`、`model_pricing_sync_test.go` 及真实调用抽样对账。

## FORK-DEPLOY-006 分支与 Zeabur 部署

- 产品目的：极速蹬在服务器完成可复现开发与验证，从 `origin/play/main` 触发 Zeabur 构建，并由用户本地电脑完成生产验收。
- 不变量：服务器使用隔离 Git worktree，业务行为执行 TDD 和逐任务规格/代码质量审查；测试、完整构建和 Fork integrity 全通过后才提交并先推送审查分支；完整 GitHub CI 只在目标为 `play/main` 的 PR 上执行一次，普通分支 push 和生产 push 不重复完整测试；确认后以非 rebase、非强推方式进入 `play/main`；只有 `origin/play/main` 触发 Zeabur；官方 Web 与 Android APK 必须能从 `jisudeng.com`、`www.jisudeng.com`、Android WebView 的 `https://localhost` / `capacitor://localhost` 安全访问 `api.jisudeng.com`，不能要求用户手填后端地址；部署 commit 和健康状态确认后，由用户本地电脑浏览器访问 `https://www.jisudeng.com/`，以游客、普通用户和管理员三种身份验收；禁止用本地服务、localhost 或服务器浏览器作为最终验收结论。
- 关键位置：`AGENTS.md`、[服务器开发与生产验收流程](./DELIVERY_WORKFLOW.md)、`scripts/push-github-and-deploy.sh`、`deploy/zeabur.template.yaml`、`deploy/config.example.yaml`、`backend/internal/server/middleware/cors.go`、`.cursor/rules/sub2api-server-only-verify.mdc`、[上游同步手册](./UPSTREAM_SYNC_PLAYBOOK.md)。
- 冲突策略：上游 Docker 文档只作为参考，不能改变极速蹬生产分支和验收入口。
- 验证：服务器完整闸门、PR 单次 GitHub CI、Zeabur 部署 commit/健康状态、官方 Web/Android 来源的 CORS preflight，以及用户本地电脑的游客、普通用户、管理员生产浏览器验收；按风险补充浅色/深色和 API/数据库对账。

## FORK-OAUTH-007 OAuth Cookie 域共享

- 产品目的：`jisudeng.com`、`www.jisudeng.com` 和 API 子域之间的 LinuxDo OAuth 流程共享临时状态 Cookie。
- 不变量：极速蹬域名返回 `.jisudeng.com`；localhost、IP 和未知域名使用 host-only Cookie；安全、HttpOnly 和 SameSite 约束继续沿用认证实现。
- 关键位置：`backend/internal/handler/auth_linuxdo_oauth.go`、`backend/internal/handler/auth_linuxdo_oauth_test.go`。
- 冲突策略：可以吸收上游 OAuth 安全修复，但必须保留域共享函数及测试。
- 验证：`TestOAuthCookieDomain`，线上分别从 apex 与 `www` 发起/完成 OAuth。

## FORK-RISK-013 IP 风险检测与批量注册发现

- 产品目的：在现有 IP 管理域内，以注册 IP 为主信号，结合登录/API IP、UA 摘要、邮箱模板、邀请码/返利码和注册后行为，主动发现批量注册与异常账号簇，并保留可解释证据供管理员复核。
- 工作台不变量：`/admin/proxies` 默认仍进入 IP 资源，并通过 `/admin/proxies/risk` 和 `/admin/proxies/actions` 提供风险检测、证据、关联账号、扫描、策略、处置预览、TOTP、部分结果和安全回滚。所有状态修改必须先生成五分钟 preview token；案件版本或处置输入变化后返回 `risk_action_preview_stale`。单次最多 500 个账号，管理员账号永远受保护，可信老账号、充值账号、历史推断账号和已禁用账号不得默认批量选择。
- 自动化不变量：迁移默认 `auto_block_enabled=false`，上线后先以 Shadow Mode 校准；启用后只允许满足精确证据、多信号族和最少注册数条件的严重案件自动创建 30 分钟注册阻止。自动动作只能针对精确 IP，IPv6 为 `/128`，只阻止继续注册，不阻止登录或正常 API 调用，不自动禁用已有账号、不停用 Key、不永久封禁。人工创建的显式注册阻止策略不依赖自动化开关，风险仓储失败时注册门控 fail-open。
- 人工处置不变量：支持观察、共享网络、白名单、临时/永久阻止注册、停用 API Key、禁用账号、解决、忽略和回滚；启用自动阻止、永久阻止、禁用账号和回滚必须使用管理员 JWT TOTP step-up。案件动作默认只处理精确 IP；IPv6 `/64` 或其他 CIDR 必须由管理员在策略管理中明确创建。回滚只恢复仍保持本次操作写入状态的用户、Key、案件或 IP 策略，不覆盖后续管理员修改。
- 检测不变量：IPv4 按 `/32`、IPv6 按 `/64` 聚合发现；未来自动资格的 IPv6 目标只能是精确 `/128`，且目标 IP 自身至少有 5 个精确注册。注册和 UA 分档各自只取最高分；历史推断、白名单、已知共享网络以及单一注册聚集信号均不能获得自动资格。共享 API IP 只统计先在候选网络注册、随后从该网络调用 API 的关联新账号。
- 隐私与保留：IP 使用 PostgreSQL `inet/cidr`；UA 保存规范化摘要和 HMAC；邮箱模板、邀请码、返利码只保存 HMAC 关联值，不保存原文。空 UA、空邀请码和空返利码不得产生可聚集 HMAC。原始事件默认保留 90 天，案件/扫描/处置审计结构默认保留 365 天。
- 历史证据：只从成功的邮箱注册审计路径 `/api/v1/auth/register` 与 `/api/v1/auth/mobile/register` 推断，并与邮箱和用户创建时间匹配；OAuth 历史不回填。推断证据只供人工查看，不得进入自动动作资格或账号默认选择。
- 告警不变量：严重风险使用现有通知邮件基础设施；同一案件只在首次达到严重等级或等级升级时发送，发送失败恢复通知 claim，避免永久漏报或重复轰炸。
- 关键位置：`backend/internal/service/ip_risk.go`、`backend/internal/service/ip_risk_service.go`、`backend/internal/service/ip_risk_admin.go`、`backend/internal/repository/ip_risk_repo.go`、`backend/internal/repository/ip_risk_repo_admin.go`、`backend/internal/handler/admin/ip_risk_handler.go`、`backend/internal/server/routes/admin.go`、`backend/migrations/214_ip_risk_foundation.sql`、`backend/migrations/215_ip_risk_management.sql`、`frontend/src/features/ip-risk/`、`frontend/src/views/admin/ProxiesView.vue`。
- 冲突策略：可吸收上游认证、审计、IP 解析和后台任务改进，但不得丢失精确注册事件、证据置信度隔离、迁移默认关闭自动化、隐私 HMAC、共享网络/白名单保护、preview/TOTP/管理员保护、安全回滚或 `/admin/proxies` 默认资源页。
- 验证：IP 风险 service/repository/middleware/auth/handler/route/migration 单元测试，真实 PostgreSQL 滑动窗口、`inet/cidr`、共享 API 新账号限定、原子案件写入和历史邮箱注册推断测试；前端路由、筛选、默认选择、扫描轮询、preview、stale、TOTP、部分结果和回滚测试。生产启用自动阻止前仍必须完成至少 24 小时 Shadow 校准。

## FORK-ADMIN-014 管理员用户批量处置

- 产品目的：让管理员在用户管理页对明确选择的账号执行批量禁用或批量软删除，同时保留逐账号结果、误操作保护和可追溯审计。
- 不变量：批量操作只接受最多 500 个明确用户 ID，并支持跨页选择；执行前必须填写原因并生成五分钟有效的服务端影响预览，预览令牌使用域隔离 HMAC 签名，用户状态或待删除 API Key 发生变化后必须返回 `USER_BATCH_ACTION_PREVIEW_STALE`。批量禁用和批量删除都必须通过管理员 JWT TOTP step-up；管理员账号永远受保护，已禁用或不存在的账号安全跳过。删除复用现有用户与 API Key 软删除事务，API Key 先去密钥化，操作完成后清理认证缓存。单个账号失败不得中断其余账号，前端必须展示完成、部分成功和失败结果，并保留失败账号选择以便重试。
- 关键位置：`backend/internal/service/admin_user_batch_actions.go`、`backend/internal/handler/admin/user_handler.go`、`backend/internal/server/routes/admin.go`、`frontend/src/components/admin/user/BulkUserActionDialog.vue`、`frontend/src/views/admin/UsersView.vue`。
- 冲突策略：可吸收上游用户管理和批量编辑改进，但不得绕过预览、TOTP、管理员保护、软删除、认证缓存失效、逐账号结果或审计原因。
- 验证：`admin_user_batch_actions_test.go`、`user_handler_batch_actions_test.go`、`api_contract_test.go`、`admin.users.spec.ts`、`BulkUserActionDialog.spec.ts`、`UsersView.spec.ts`，以及前端 typecheck、lint、design governance、完整测试和 production build。

## FORK-PUBLIC-008 公共页面与可见性

- 产品目的：游客可浏览极速蹬首页、文档、模型价格和 Play 展示，登录用户看到与账号分组匹配的内容。
- 不变量：`/home`、`/models`、`/docs` 和公开 Play 页面存在；`public_models_enabled` 控制游客目录；`available_channels_enabled` 控制登录价目数据而不是恢复用户侧栏“可用渠道”入口；公开接口不泄露渠道密钥、账号或内部定价配置。
- 关键位置：`frontend/src/views/public/`、`frontend/src/content/public-docs-data.zh.ts`、`backend/internal/service/public_model_catalog.go`、`backend/internal/service/setting_public.go`。
- 冲突策略：上游公共设置字段变更需要合并到 DTO 和前端类型，但极速蹬可见性语义优先。
- 验证：public model/catalog、public settings 和 teaser tests；游客、登录用户、管理员三种身份线上检查。

## FORK-MIGRATION-009 自定义数据库迁移

- 产品目的：保留 Play、品牌默认值、图像工作室、提示词库和模型目录的数据库结构与数据修复。
- 不变量：下列文件名完整存在且已应用文件不可改写；上游出现同数字前缀时允许并存，不能按编号覆盖，例如上游 `181_prompt_audit.sql` / `182_prompt_audit_full_prompt.sql` 与 Fork `181_jisudeng_public_model_pricing.sql` / `182_image_studio_asset_storage.sql` 必须同时保留。runner 在固定的同一 PostgreSQL session 上获取 advisory lock、执行迁移并校验解锁结果；192/194 的表变更按 runner 白名单分成可恢复短事务阶段，长 backfill/constraint validation 不携带前置 `ALTER TABLE` 强锁；对应 `_notx.sql` 索引继续使用 `CONCURRENTLY`。
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
192_image_studio_persistent_jobs.sql
192_image_studio_persistent_jobs_indexes_notx.sql
193_image_studio_references.sql
194_image_studio_asset_derivatives.sql
194_image_studio_asset_derivatives_indexes_notx.sql
195_image_studio_billing_reconciliation.sql
196_image_studio_job_references.sql
197_image_studio_object_deletions.sql
198_image_studio_upload_slots.sql
199_prompt_library.sql
200_prompt_library_seed.sql
201_prompt_library_public_seed.sql
202_prompt_library_generic_cover_cleanup.sql
203_batch_image_owner_idempotency.sql
205_vip_recharge_bonus_snapshot.sql
206_vip_recharge_legacy_tiers_backfill.sql
207_balance_transactions.sql
207_subscription_plan_product_display.sql
208_image_studio_asset_lifecycle.sql
208_image_studio_asset_lifecycle_indexes_notx.sql
209_play_arena_daily_reward_summary.sql
210_subscription_plan_storefront.sql
210_withdrawable_entitlements.sql
211_withdrawals.sql
212_withdrawals_integer_amounts.sql
213_fund_management_batches.sql
214_ip_risk_foundation.sql
215_ip_risk_management.sql
```

## FORK-BILLING-010 计费归属与充值联动

- 产品目的：防止 API Key、订阅或批量任务被错误归属到其他用户，同时让成功充值触发可选 Play boost。
- 不变量：扣费前验证 API Key 与用户归属；订阅扣费验证订阅所有者；余额冻结同样验证归属；NextChat Web 和 Android 只能通过登录用户 JWT 换取该用户名下的受管 API Key，并按用户允许分组返回模型/余额/API Key 权限，不能暴露其他用户密钥；粘性会话种子按 API Key 隔离；模型目录参考价不能覆盖真实渠道计费；支付订单完成后再授予 recharge boost，boost 失败不得回滚已完成充值；`frozen_balance` 只代表图片/任务预留，提现冻结必须写入独立 `withdrawal_frozen_balance`；可提现权益通过 `withdrawable_entitlements` 和 immutable allocation 流水对账；用户提现默认关闭，只有 ready 用户可启用；提现申请必须锁定成熟权益批次，取消、拒绝和退款必须恢复原批次；提现金额和提现规则金额必须为整数；充值退回必须走独立 `balance_fund_batches` / `fund_refund_requests` 批次和审核流程，真实线上/线下充值可退未消费整数部分，赠送/首 30/兑换码赠送默认不可提现也不可退；管理员审批、读取完整收款资料和线下打款登记必须使用 JWT 管理员 TOTP step-up，管理员 API Key 禁止。
- 关键位置：`backend/internal/repository/usage_billing_repo.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/gateway_service.go`、`backend/internal/server/routes/nextchat.go`、`backend/internal/service/payment_fulfillment.go`、`backend/internal/service/play_recharge_boost.go`、`backend/internal/service/withdrawable_ledger.go`、`backend/internal/service/withdrawal.go`、`backend/internal/service/fund_management.go`、`backend/internal/service/fund_batches.go`、`frontend/src/views/user/WalletView.vue`、`frontend/src/views/admin/AdminWithdrawalsView.vue`、`frontend/src/views/admin/AdminFundsView.vue`。
- 冲突策略：上游支付状态机和安全修复必须合入；归属校验、真实计费优先级与充值后 Play 联动必须保留。
- 验证：usage billing unit/integration tests、session hash tests、model pricing tests、payment lifecycle tests、NextChat mobile bootstrap/group switch route tests；线上以测试订单检查余额到账和 boost 状态。

## 更新规则

1. 新增或改变 Fork 行为时，先更新对应条目；没有对应条目时创建新的稳定 ID。
2. 条目必须至少绑定一项静态检查或行为测试；高风险计费、认证和权限行为必须有测试。
3. 上游同步完成并验证后，统一更新本文顶部基线及受影响文档日期。
4. 删除定制必须在同一 PR 删除登记、保护检查和专属测试，不能只删代码。
