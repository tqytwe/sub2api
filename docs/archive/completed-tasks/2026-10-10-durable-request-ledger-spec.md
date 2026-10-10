# 持久请求台账（实施规格）

基线：`play/main@2fbe13d90a2381a3bc8e7495a0c8b00abe2cfe3a`。
工作分支：`codex/durable-request-ledger-20261010`。本任务不发布生产。

## 不变量

- 受管入口在鉴权、限流、请求体读取之前，生成服务器私有 UUID 并同步提交 PostgreSQL。
- UUID 与客户端 request ID、上游 ID、计费幂等键分离，绝不采用客户端提供的身份作为台账主键。
- 每次本应用直接执行、可能产生上游副作用的发送（包括兼容性重发）之前，必须提交对应 attempt。
- WS 连接和每个 turn 分别建账；异步提交和后台执行分别建账并保留父子关联。
- 鉴权成功同步关联 user/key ID；鉴权失败保存匿名拒绝。仅存数字 ID，不保存 key 或其哈希。
- attempt 保存调度 account ID 与实际采用的 credential 母账号 ID；非影子账号二者相同。
- 不保存请求/响应正文、Authorization、Cookie、IP、UA、原始 URL/query 或任意上游错误文本。
- 错误只用固定安全枚举。路由只存已注册的模板；未知受管路径用固定模板 `unmatched_gateway`。
- 不新增扣费动作，不估计 token/消费，不回填历史，不把无 usage 当成零费用。

## 状态与最小数据模型

`gateway_requests`：私有 UUID、parent UUID、kind（HTTP/WS连接/WS turn/后台执行）、turn 序号、
路由模板/方法、起止时间、user/key ID、实例 UUID、执行状态、用量状态、结算状态、安全错误码、
HTTP 状态。三条状态轴互相独立：

| 轴 | 状态及含义 |
| --- | --- |
| 执行 | inflight / succeeded / failed / cancelled / timeout / interrupted |
| 用量 | not_applicable（未发送或明确无需用量）/ pending / known / usage_unknown |
| 结算 | not_required / settlement_pending / settled |

`gateway_request_attempts`：`(request_id, attempt_no)` 唯一，调度/凭据 ID、开始/结束、执行状态、
上游响应状态、安全错误码。序号由数据库分配。开始仅证明“发送已获准且可能已发生”，不能
声称 PG 提交与外部网络发送之间存在分布式原子性。

`gateway_ledger_instances`：实例 UUID、PG 时钟租约。只有过期实例的请求可回收，不能在启动时
无差别终结所有 inflight。恢复仅标 interrupted/usage_unknown/待核对，不重新发送、不补扣。

计费引用通过现有 request ID + API key ID + verified fingerprint 关联 usage_logs、dedup/归档、
balance_transactions、subscription/package entitlement；只有可验证结算证据才显示 settled。
台账终态重复写入幂等，迟到的 usage/结算证据可补充另外两条状态轴，不把执行失败改写成功。

## 存储故障与影响（逐请求策略）

1. 入口写入失败：503 + Retry-After，固定安全错误码 `request_ledger_unavailable`，不鉴权、不选路、不发上游。
2. 身份/attempt 写入失败：停止当前请求的后续上游发送；未输出 HTTP 返回 503；已升级 WS 安全错误并终止该 turn。
3. 上游发送后终态写失败：已提交入口和 attempt 保留；通过租约恢复为 interrupted、待核对。
4. 不修改生产配置，不全局关闭平台，不依赖 Redis 或 best-effort 日志队列作为唯一凭据。
5. 增加每个入口、鉴权绑定和每次发送的 PG 写入与延迟；匿名流量也占持久容量。同实例入口会更新一行 lease，存在写入热点；生产吞吐/容量尚未压测，不作吞吐承诺。
6. PG 无法写入时无法在同一 PG 保存新的拒绝记录；不会声称 100% 永不丢。TLS/HTTP 解析前、
   反向代理拒绝、机器或所有持久介质同时损坏在应用台账能力边界之外。

## 受管路由清单

以下以基线 `gateway.go`、`router.go`、`image_studio.go`、`nextchat.go` 核对；具体注册矩阵
须由测试与实际 Gin 路由表比对，新增路由不能静默漏掉。入口中间件覆盖下述路由；失败/取消与正常协议的证据层级见验收矩阵，不能把路由拒绝测试当成所有协议成功测试。

| 前缀 | 方法与后缀 |
| --- | --- |
| `/v1` | POST `/messages`, `/systemone`, `/messages/count_tokens`, `/responses`, `/responses/*subpath`, `/alpha/search`, `/chat/completions`, `/embeddings`; GET `/responses`（WS）, `/models`, `/models/:model`, `/usage`, `/sub2api/billing` |
| `/v1` | POST `/images/generations`, `/images/edits`, `/images/generations/async`, `/images/edits/async`; GET `/images/tasks/:task_id`, `/images/task-assets/*filepath`, `/images/results/:result_id/:index` |
| `/v1` | POST/GET `/images/batches`; GET `/images/batches/models`, `/images/batches/:id`, `/images/batches/:id/items`, `/images/batches/:id/items/:custom_id/content`, `/images/batches/:id/download`; POST `/images/batches/:id/cancel`; DELETE `/images/batches/:id`, `/images/batches/:id/outputs` |
| `/v1` 及空前缀 | POST `/videos`, `/videos/generations`, `/videos/edits`, `/videos/extensions`; GET 各 `/videos/{generations,edits,extensions}/:request_id` 及 `/content`, `/videos/:request_id` 及 `/content`, `/agnesapi` |
| `/v1` | POST `/audio/speech`, `/audio/transcriptions`, `/audio/translations`, `/live`; GET `/live/:call_id` |
| `/v1` 及空前缀 | POST `/tts`, `/stt`, `/custom-voices`; GET `/custom-voices`, `/custom-voices/:voice_id`, `/custom-voices/:voice_id/audio`, `/realtime`; PATCH/DELETE `/custom-voices/:voice_id` |
| `/v1beta` 及 `/antigravity/v1beta` | GET `/models`, `/models/:model`; POST `/models/*modelAction`（generateContent/streamGenerateContent/countTokens 等动作） |
| 空前缀 | POST `/responses`, `/responses/*subpath`, `/alpha/search`, `/chat/completions`, `/embeddings`, `/messages/count_tokens`, `/images/generations`, `/images/edits`, `/images/generations/async`, `/images/edits/async`; GET `/responses`, `/models`, `/models/:model`, `/images/tasks/:task_id`, `/images/task-assets/*filepath`, `/images/results/:result_id/:index` |
| `/backend-api/codex` | POST `/responses`, `/responses/*subpath`, `/alpha/search`, `/realtime/calls`; GET `/responses`, `/models`, `/:call_id` |
| `/api/v3`, `/v3`, `/v1`, 空前缀 | POST `/contents/generations/tasks`; GET/DELETE `/contents/generations/tasks/:task_id` |
| `/antigravity` | GET `/models`; `/antigravity/v1`: POST `/messages`, `/messages/count_tokens`; GET `/models`, `/usage` |
| `/api/v1/image-studio` | GET `/templates`, `/capabilities`, `/models`, `/estimate`, `/jobs/active`, `/jobs`, `/jobs/:id`, `/jobs/:id/download`, `/assets/:id/{thumbnail,content,download}`; POST `/generate`, `/references`, `/jobs/:id/cancel`; DELETE `/references/:id`, `/jobs/:id` |
| `/api/v1/nextchat/image-studio` | 同上鉴权工作区路由（不含公共 templates/capabilities），采用现有 BFF 身份验证 |
| `/api/v1/mobile` | `/tasks` POST/GET、`/tasks/:id` GET/DELETE、`/tasks/:id/{cancel,retry,status}` POST；`/image-history` GET、`/image-history/:id` DELETE、`/image-history/:id/retry` POST；`/web-search` POST |
| `/api/v1/mobile/video` | GET `/bootstrap`, `/models`, `/jobs`, `/jobs/:id`, `/jobs/:id/content`; POST `/estimate`, `/jobs`, `/jobs/:id/{cancel,retry}`, `/jobs/:id/content/ack` |

模型/额度/计数等非生成查询同样留台账，但不因此扩大收费策略。前端 HTML 重定向不假装模型生成。
后台图像批次 item、异步图像、Image Studio、移动视频 worker 必须携带持久父请求关联。

## API 与界面

- 普通用户 list/detail/attempts 在 SQL 中强制 user_id；参数不能覆盖该范围。
- 管理员按 user/key/account（含母账号）/三轴状态/date/private ID 筛选。
- 使用有界分页、稳定时间/ID 排序。没有 usage 时金额显示未知，不能显示 $0。
- 沿用用户和管理用量页、共享 DataTable/Select/按钮/详情浮层，显示 inflight、失败、待核对和计费状态。
- 关联 usage 页面可追溯；普通用户 DTO 不暴露其他用户或上游凭据账号详情。
- 无清理或删除 API；旧 usage 清理不级联删除新台账。

## 迁移、阶段与验收

新增 forward-only SQL，`SET LOCAL lock_timeout='2s'`、`statement_timeout='30s'`。新空表索引同事务；
若涉及既有大表索引则独立 `_notx.sql` + CONCURRENTLY。无历史更新、无级联删除、无旧 migration 修改。
根 AGENTS 引用的 PROJECT_HYGIENE.md 与 ZEABUR_POSTGRES_RUNBOOK.md 在基线不存在，按现存
DELIVERY_WORKFLOW.md、migrations/README.md 和已上线 274 迁移约束执行。

可独立审查阶段：入口+DB+查询+恢复 → 全部 attempt/turn/任务+计费引用 → 前端与完整验证。
前一阶段不得宣称后续覆盖已完成。完成前必须有真实 PG 迁移、HTTP/API、前端刷新证据；
矩阵包含 401/429/400/500/超时/断流/取消、多轮 WS、发送后进程终止、结算失败、重启恢复、
重复终态不重扣、角色权限/分页、生命周期及保留，并运行仓库完整门禁。

## 并行文件交集

- 路由线程：`server/router.go`、`service/{openai,gateway,gemini}_upstream_transport_error.go`；添加早期台账及存储故障直接返回，不改变选路策略。
- WS usage 线程：`handler/openai_gateway_handler.go`, `service/openai_ws_forwarder*.go`,
  `service/openai_ws_v2/passthrough_relay.go`；仅身份/发送门/终态调用，不重做 usage 快照。
- 额度查询线程：`server/middleware/api_key_auth*.go`；只在鉴权已验证后持久绑定，不改收费豁免。
- 缓存 PR345：可能涉及 HTTP transport；仅记录真实尝试，不改变缓存、HTTP2 或重试策略。
- 新包与新 API 尽量独立；最终由根线程普通 merge 组合，禁止强推。

## 实际范围与独立交付边界

当前合入主线基线：`7291ae9a2be4db7d97b8b641d053f7822276dc23`（普通快进，包含缓存、只读额度及 PR346）。
PR346 原 head 为 `edee1a0a57ccfaf0044c1c2edd4c1da6062854e6`；按根线程授权合入，没有发布。持久入口、发送门、恢复、账务引用和页面为同一兼容增量。
新表不依赖旧版本写入；回滚旧二进制会停止新增台账，既有台账保留，不能把回滚期描述成仍有完整覆盖。
滚动发布期间仍在服务的旧实例也不会产生台账；覆盖起点必须以全部受管实例升级及入口核验为准，不能用迁移完成时间代替。

| 独立切片 | 已有实现 | 验收边界 |
| --- | --- | --- |
| 入口 → PG → HTTP API → 页面 | 先提交私有 ID、鉴权绑定、三轴状态、租约恢复、只读分页/过滤 | 真实 PG 全迁移、实际 handler API、合成身份浏览器；不是生产验收 |
| HTTP/SSE/Responses WS | 每次可见发送前提交 attempt；实际入站 WS 两 turn，各自 usage 状态 | HTTP/SSE 截断/超时/取消、原生 Responses 服务路径；没有声称所有供应商端到端完成 |
| 账务引用 | 在既有事务中验证 dedup，与 wallet/subscription/package/usage 原记录关联 | 故障注入证明余额/去重一起回滚，重试不重扣；台账不发起补扣 |
| 异步图像/批次/视频 | 提交先绑定任务，worker 新请求关联持久父记录，wallet capture/release 只读恢复 | 批次真实 fake HTTP、移动视频 provider、真实钱包恢复；不是完整第三方任务生命周期实测 |
| Realtime/Live | 显式 response.create 先记 turn，输入流先记 attempt，服务端自动 turn 观察时记独立身份 | 真实 PG 状态机测试及已有协议测试；自动 turn 的触发身份不能在收到事件之前知道 |

### 明确未证实或无法观察的范围

- Realtime 自动 turn、Live sideband、Grok 双向流尚无全部模式的真实服务端端到端矩阵；现有测试不提升为全协议验收。
- WebRTC 直连媒体不经过网关；入口及 sideband 有证据，但无法证明每个直连媒体数据包/模型内部自动重试。
- 插件调用前有持久 attempt，插件进程内部重试不在当前应用边界内；不能虚构内部 attempt。
- 上游已发出的字节记为 `output_observed`，不声称客户端已收到；每个 attempt 独立保存该标记和 usage 状态。
- 第一次失败消费未知、第二次成功有 usage 时，请求聚合仍为 `usage_unknown`；可显示已证实的用户扣费引用，但不能推算总上游损失。
- 自动 turn 先有已提交的连接/输入因果证据，收到创建事件后补独立私有身份。必须与“明确客户端 turn 发送前已有身份”区分。
- 已有按会话聚合的实时收费保留在会话/执行父记录，不把总额虚构分摊给各 turn；没有逐 turn 结算证据的子记录保持待核对，需连同父记录审阅。
- HTTP 协议解析、反向代理或机器故障发生在应用接纳前时不在本台账内；PG 及所有副本同时不可用时无法无限保证保存。

### 保留与运维影响

本迁移没有删除、TTL、清理任务、历史回填或旧表外键级联；后台恢复只更新已存在行，最多每轮 1000 行。
钱包核对按最后检查时间轮转最多 1000 行，使用既有 capture/release 与 dedup 证明；hold 本身不算结算。
匿名请求同样增长数据量，增加每请求与每 attempt 的同步 PG 写入；需要根线程发布前评估磁盘、连接池和写入延迟。
本分支不改生产保留策略、限流或配置；后续归档/容量治理需另行设计，不能自动清旧账。查询限页、限大小及超时。

### PR346 合并接口

`handler/openai_gateway_handler.go` 的台账终态 defer 必须位于 `turnSettlement.claim` 提前返回之前。
复用 PR346 `bindAttempt` 的逻辑 turn 编号；使用 `turnSettlement.context(ledgerCtx, turn)` 保留私有台账 context 和原计费幂等键。
不得复制该 PR 的快照、claim 或收费算法。已在指定主线普通快进后人工合并上下文接口，重新验证缺失 ID 终态与重试组合；未修改 PR346 的结算算法。

状态：实现及本地必需门禁已完成，证据见最终审查记录；根线程外部组合审查、生产发布与用户本地验收待完成。生产历史缺口保持未知。
