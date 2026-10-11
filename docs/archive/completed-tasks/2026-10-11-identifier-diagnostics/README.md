# 上游标识冲突诊断后端小批

状态：正式主线359组合合同/race、按序双审及全部本地门禁通过，待独立草稿提交；未部署。
独立分支 `codex/identifier-diagnostics-20261011` 最初在
`6ae024e6b732b64b2ce6f7ba56c5be4e20b88bdb` 独立实现，随后正式主线已合入359，
本分支保存未提交改动后普通快进至 `8364b462feda15f3bf8f3bb2029314d2193b9dba` 并恢复。
279与主线worker测试修复原样继承，不作为本批新增改动；不引入未完成280。

## 问题与范围

原台账仅保留首个上游 request/response ID 和合并 conflict 标记，无法说明后续冲突值
来自哪个已观察字段。本批在原精确 Receipt 上记录未来的有界来源快照，保留原首值、
真实同 namespace 冲突与 attempt 归属。不同重试的 ID 不合并，迟到 Ops 和恢复旧 response
仍使用原 Receipt，不根据账号、时间邻近或“当前 attempt”猜关联。

实际图片适配器的 `response.incomplete` 与 `response.failed` 曾把 `response.id`
放入 `UpstreamRequestID`。真实 HTTP/PG RED 已复现，新增独立 `UpstreamResponseID`
并让 request ID 继续来自真实响应头。公开错误、状态码、重试和调用次数保持不变。
此问题不证明生产16条普通 Responses 流式冲突的根因；不清历史 flag、不回填来源、
不重放请求，也不推断上游计费或补扣。

## 快照与丢失语义

`identifier_snapshot_v1` 每个 request/response namespace 只留首值和第一次不同值，
后续值或重复相同值不覆盖其来源、时间。只存私有 attempt UUID、既有最大128字符的 ID
白名单、受控 source/field/event_type 枚举及本地观察毫秒时间；未知元数据映射为 unknown。
不保存 header/body 原文、URL、凭据、extra 或任意错误文本。原 header/JSON字段选择优先级
与64KiB解析上限保持不变，不额外遍历任意嵌套字段。

复用既有256项非阻塞异步队列、250ms context、连接池至少一个连接的预留检查和
`FOR UPDATE SKIP LOCKED`。快照在原锁内编码为不可变JSON，搭原异步UPDATE写入，
精确核对 request/attempt_no/attempt UUID；不增加业务同步数据库往返或诊断重试。
数据库仍以COALESCE保留首值、OR保留真实冲突，旧writer不能清除新快照。

快照不提供“完整观察”的证明：首值持久化后仍可能丢掉后续冲突，所以顶层
`completeness` **始终为 `unknown`**。`identifier_conflict=false` 只代表持久化标记，
不能显示成“无冲突”。缺 evidence 行时 attempt UUID 与 conflict 均为null。

| status | 含义 |
| --- | --- |
| persisted_snapshot | 有界快照版本、私有UUID、首值、枚举和已有flag一致；后续仍可能丢失 |
| not_recorded | 空对象或缺证据行；可能是旧writer、尚未落库或丢失，不能据此区分原因或解释为零冲突 |
| incomplete | JSON无效/过大、归属或首值/flag不一致，不能伪造旧来源 |
| unsupported_version | 明确非空且无法解释的版本；缺失/空/null版本属于incomplete，不透传未知内容 |

HTTP SSE/JSON的event_type仅表示payload.type；SSE缺type时为unknown，不采集独立
`event:`行或将前帧类型套给后帧。HTTP JSON无type为none，Ops使用封闭kind枚举。
只覆盖程序实际观察的传输头、HTTP SSE/JSON、精确Ops回灌；既有通用观察器以
legacy_observer/unknown保留。版本是快照格式，不是全协议覆盖或无丢失承诺。
终端后仍观察到不同ID可保留证据，但不改变原 failed/completed 终态。

## 管理员 API

`GET /api/v1/admin/requests/:id/identifiers?after_attempt=0` 为独立只读子资源，
复用管理员权限及私有请求UUID。匿名401、普通用户（含请求所有者）403、无效UUID或
游标400、不存在404、证据查询失败仅此子资源503；旧请求详情仍可用，结果no-store。

按 attempt_no seek，先取最多129个来源候选，返回最多128项，额外一项只判断has_more。
`next_after_attempt` 为末项序号，无后页则null。游标不代替权限；实时新证据可能出现在
已读 attempt，需从头刷新，不声称固定快照。reader只返回白名单DTO，未知额外JSON字段
不会透传；证据查询使用既有100ms可选读取期限。没有前端接线或用户界面完成声明。

## 迁移与兼容

新增 `281_gateway_identifier_diagnostics.sql`，仅给现有 attempt evidence 表增加
默认空JSONB列和4096字节/object约束。275–280不改；不创建279/280占位或额外表。
`ACCESS EXCLUSIVE NOWAIT` 在改表前拒绝父表读写争用，其他锁50ms、单语句500ms；
NOT VALID避免扫描历史行，但仍约束新写入。实际取得锁后的DDL/提交/I/O开销仍存在，
不是端到端零阻塞或硬实时保证。

真实迁移运行器RED移除这三个时限后，读写争用等至调用截止，慢DDL错误地成功。
恢复后应立即55P03，慢DDL57014，整个事务及迁移登记回滚；解除争用后重试，重复启动
校验和一致。旧INSERT默认{}、旧UPDATE保留快照与首值/flag；历史数据不重写。
应用回退保留新增列和已存证据，不执行down migration、删除或回填。

## 验证记录

TDD记录见[namespace-red.json](./namespace-red.json)、[migration-red.json](./migration-red.json)
及[progress.json](./progress.json)。快照API先缺失而RED；reader曾将缺证据行误判incomplete，
真实HTTP分页断言发现后修为SQL NULL/not_recorded，保留原失败。初次测试夹具漏绑定身份
或credential_account_id的失败也保留，不写成产品根因。

初始HTTP/PG已覆盖：重试/辅助请求与迟到原Receipt、两namespace首个不同值、terminal后ID、
JSON选择、旧flag、队列满/行锁/池忙、首快照后真实子进程kill、128项分页、权限与秘密哨兵、
查询故障隔离，以及281实际runner争用/执行超时/回滚重试/新旧writer。补充race与旧关联
回归在原6ae基线上通过：冻结台账合同76/76、race共506项测试/子测试通过，4个条件子进程/浏览器夹具跳过；核心合同54/54、203项通过。
原全量make test的后端与lint已通过，前端因新工作树缺eslint而停止，失败保留于
[base-full-first-failure.json](./base-full-first-failure.json)。已从现有缓存离线安装锁定依赖，未改锁文件。

主线合并保留75项必需台账合同，合并本批20项并新增同请求路由Finish/迟到Receipt组合回归，
最终清单96项。组合测试验证路由complete与标识完整性unknown同时成立，失败候选及精确attempt归属保留，
终态、HTTP次数与usage_unknown不变。22个源码/CI输入冻结见[tested-source-sha256.txt](./tested-source-sha256.txt)，
原21输入清单另存；18个非共享本批输入未变。406份现有迁移、58份非共享主线文件与所有其他handoff内容
保持原样，见[main-sync.json](./main-sync.json)。组合合同/race结果见[merge-validation.json](./merge-validation.json)：台账96/96必需用例、129顶层/564测试及子测试通过，3个条件子进程入口与2个可选浏览器夹具跳过；核心54/54必需用例、203测试及子测试通过，无跳过。完整 `make test`（后端全量、lint 0 issues；前端lint/类型检查与504文件3825项通过、8项既有跳过）、`make test-backend-unit`、`make build`、embed构建、完整Fork检查均通过；22项源码/CI输入哈希未变。

规格审查两项P2经RED/GREEN关闭后复审通过，再完成独立质量审查，无剩余P1/P2；
见[reviews.json](./reviews.json)。仓库完整测试、构建及Fork检查已通过，仅推送独立草稿PR并检查最终head CI。
主线程负责合并部署；生产、用户本机三身份验收和后续UI均未执行。
