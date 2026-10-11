# HTTP 文本路由证据后端小批

状态：本地后端门禁已验证，独立草稿未合并/部署；维护责任：本任务提交者。生产发布由主线程协调。

基线 `23d06337cbc315cb92d3fc18e94902fe21e7342d`；独立分支
`codex/routing-evidence-20261011`。发布前已普通 fast-forward 合入
`05b44bec5c2d30ad58d009fbe5247ea363c6ee70`，保留 #352 全部修复；无 rebase/force。

## 证据口径

`coverage_version=openai_compatible_http_text_v1` 只覆盖 OpenAI-compatible
HTTP Responses、Messages、Chat 与现有别名的实际高级/legacy/sticky/previous-response、
复检、抢槽、fallback 和终端准入路径。只记录程序实际执行的首个 veto，保留原有短路、
排序/RNG、Acquire/Release、重试和粘性行为。`selection_no` 是调度调用序号，与上游
`attempt_no` 独立；成功请求也保留此前被拒候选。fail-open 读取失败是观察事件，
不增加 veto_count。首个 veto 按每次实际评估计算，同一账号后来重新评估可以另有记录。

候选源的仓库/快照已经过滤账号；`prefiltered_schedulable_pool` 的 value 是该源返回数。
`source_population_complete=false` 始终保留，pool=0 不能证明分组无账号，也不能解释源中
不可见账号的历史状态。没有额外历史配置读取或候选谓词重跑。

Anthropic/Gemini 实际分发、媒体及 WS/子轮不属于这一 coverage version；它们返回
`unsupported_coverage`，鉴权前拒绝等没有开始选路的请求返回 `not_recorded`。
不宣称 HTTP 模型/计数查询、全协议、UI 或历史根因已经覆盖。

用户提供的固定 07:04:37–07:19:37 UTC 窗口 27 条 OpenAI HTTP Responses 请求在
upstream attempt 前选路503；当时没有每候选排除原因，配置随后变化。
本批不倒推根因、不称已证实路由算法缺陷，不重放、删除、回填历史或补扣。

## 存储和失败契约

迁移 `279_gateway_routing_evidence.sql` 只增 `gateway_requests` 两列和独立
`gateway_request_routing` 表；275–277 原字节不变，278 留给并行分页。
旧 writer 省略新字段得到空 version / not_recorded；新 reader 不把旧记录解释成零拒绝。
迁移使用5秒锁等待、30秒语句超时；迁移失败由既有运行器事务回滚，修复锁争用后重试。
应用回退可保留新增列/表和已存证据，不执行 down migration 或清空数据。

版本与结束摘要搭现有 Begin/Finish SQL，不新增请求同步 DB 往返。
值类型 DTO 仅包含 selection_no、stage/reason 枚举、必要账号 ID、已知池大小与时间；
异步批次只带私有请求 UUID 和不可变 JSON。拒绝未知枚举；不存 URL/header/body、extra、
凭据、模型原文或任意 err.Error()。
每请求最多128条且32KiB，超限计 omitted_count。结束后非阻塞进入64批队列；一个 worker，
Finish 原 SQL 结束后释放写入屏障，200ms context、50ms lock_timeout、150ms statement_timeout，
受限池保留至少一个连接空间。队列满/已停止不会等待或改变准入/结算；锁忙/池忙/崩溃
可以丢明细，但查询必须明确缺证据。

| 状态 | 含义 |
| --- | --- |
| complete | 在本 coverage 内摘要与已存有界事件数一致；不表示全分组账号都被观察 |
| truncated | 已持久化有界事件，omitted_count>0 |
| pending | 请求未结束或明细仍在异步等待窗口（结束后20秒） |
| incomplete | 已知丢批/未知枚举/崩溃、缺结束摘要或等待窗口后仍缺明细 |
| not_recorded | 旧 writer、未开始已覆盖选路，不能当作无拒绝 |
| unsupported_coverage | 实际路径不在本 coverage 内 |

## 管理员查询

`GET /api/v1/admin/requests/:id/routing` 复用现有管理员鉴权和私有 UUID。
普通用户403、匿名401、不存在404、UUID无效400；成功结果 no-store。
查询失败仅此子资源503，已有管理员/用户请求详情仍可读。
新 `routing_query.go` / `routing_handler.go` 独立于并行分页拥有的
`query.go` / `request_ledger_handler.go` / 详情 UI；未修改账号管理、auth 或 usage/wallet 结算。
JSON返回 request_id、coverage_version、结束摘要及 events（无事件时为空数组）。
尚无 UI 接线；后续调用者必须展示 coverage 和缺证据状态，不能把空 events 显示为无拒绝。

## 验证与审查

TDD：捕获器先因未定义 API 编译失败；实际 HTTP scheduler 测试在无 hook 时等待证据失败，
实现后通过。扩展测试发现并修复 missing sticky 双计数；保留实际谓词和动作顺序。
规格审查发现的缺失 Grok 枚举、hydration/compact 分支、未覆盖 provider 分发与 fail-open
计数已修复。质量审查要求补齐 Finish/worker 三类时序的真实 PG 验证，已补。

定向测试覆盖真实 HTTP 三处理器、三调度模式、失败候选后成功、复检/槽位/fallback动作计数
与未开启观察时一致、pool 预过滤、归属权限/独立查询故障、秘密哨兵、128条限制、旧 writer、
真实子进程 kill、迁移锁、队列满/表锁/连接池忙、Finish barrier/失败与最终缺批状态。
生产和用户本机验收未执行。完整门禁结果见本目录 validation.json（完成检查后更新）；
CI 只由独立草稿 PR 触发一次，不合并或部署。

### 最终本地门禁

| 检查 | 结果 |
| --- | --- |
| 独立规格/质量复审 | 均通过；首轮问题及 Finish/worker 时序缺口已修复复审 |
| make test | 通过：全后端、lint 0 issues、前端 lint/typecheck、499文件/3601测试通过；既有3文件/8测试跳过 |
| make test-backend-unit | 全量通过 |
| make build / embed build | 前后端完整构建与包含生产静态资源的后端均通过 |
| Fork integrity | 完整静态与受保护行为检查通过 |
| 台账 HTTP/PG 合同 | 64/64 必需项通过，无失败 |
| 核心迁移 PG/Redis 合同 | 54/54 必需项、203测试/子测试通过，无跳过 |
| #352 组合套餐/订阅合同 | 真实 HTTP/PG 回归通过，未改其结算代码 |
| routing race | requestledger/acceptance/handler/routes/service通过；冻结源码service另复跑通过 |
| 文档/交接/源码清单 | 本地链接、交接结构及8项测试、diff检查、25文件校验清单通过 |

本地工具链 Go1.27.2、golangci-lint2.14.0、pnpm9.15.9；PR工作流自行使用其固定工具链。
此前计时断言波动、lint锁冲突、静态检查修正及16GiB环境OOM记录保留于
[prior-failures.txt](./prior-failures.txt)。最终单独执行make test时限制Go内存4GiB通过，
没有删测试、放宽断言或手工终止检查。精确结果见[validation.json](./validation.json)，
源码对应[tested-source-sha256.txt](./tested-source-sha256.txt)。CI、合并部署和用户本机验收另行记录。

技术验证提交：`0702e5bf41018a1aef8b8f9ff4fcb931ddfc4160`；后继提交仅补交接元数据，源码校验清单不变。
