# 持久请求台账：规格与质量复核记录

范围：独立 worktree `codex/durable-request-ledger-20261010`，初始审查基线 `3843ff3e931349595b8793b52504b02a177f12c9`，后按根线程授权快进到 `7291ae9a2be4db7d97b8b641d053f7822276dc23`，再按根线程集中授权普通 merge `a581d8db536157c6c325ea4729381ee8b83ed53d`。
本记录是该实施线程的两轮检查，不冒充外部 reviewer 的独立批准。根线程负责组合审查、发布和生产验收。
规格见 [台账规格](./2026-10-10-durable-request-ledger-spec.md)。

## 第一轮：规格检查

- 私有服务器 UUID 与客户端 ID、上游 ID、收费幂等键分离；入口在 auth/body/CORS/路由 handler 前同步 PG 提交。
- 已验证 auth 才写数字 user/key ID；匿名拒绝也保存。前端普通用户范围在 SQL 里强制，管理员另有角色检查。
- 每个应用可观察的上游发送有预提交 attempt；实际 HTTP 重试位于兼容 transport 内层，调度/母账号分开。
- WS turn、连接/control、输入流和自动观察 turn 的阶段明确区分；异步入队意图与后台执行分别保留父子关系。
- 执行、用量、结算独立；不把缺失 usage 改成 0，不因成功重试覆盖之前 attempt 的未知消费。
- 已有 settlement/dedup、usage、钱包、订阅及套餐引用保留；台账不产生新的收费策略或金额。
- recovery 把过期实例及本实例已结束处理但终态写失败的记录标记中断/未知；不重发、不补扣、不补历史，不删除。
- 受管路由拒绝覆盖不等于完整供应商协议成功覆盖。Realtime/WebRTC/plugin 内部的观察边界在规格逐项标注。

发现并修复：usage worker detached context 会失去私有请求归因；SSE 明确失败终态会被后来的通用成功标志覆盖（可恢复 bare error 由业务 parser 裁定，不一概视为最终失败）；
第二笔 billing intent 会错误继承上一笔 settled；成功 retry 会隐藏第一次未知消费；图片幂等重放未关联实际旧任务；
管理员/用户请求路由漏加 lazy locale scopes；英文筛选丢失语言参数；流式响应已写入 200 后 panic 会被误记为 HTTP 500；Anthropic/Gemini 会把台账故障转为上游 failover；Realtime 跟踪容量拒绝在独立 turn 持久化之前发生（两类 PG 用例 RED→GREEN，05:54:52 UTC exit 0）。
合入 PR346 后追加发现：原始 terminal 不能自行结束 passthrough 当前 attempt；迟到的 WS usage 不能标记重试后的最新 attempt。两项真实 PG RED 于 06:05:15 UTC 证实，按 relay 权威 turn 回调和精确 attempt 编号修正，未复制响应 ID 归因或收费算法。GREEN 于 06:08:31 UTC：service 21.550s、ledger 4.553s；随后统一执行合并后的最终门禁。

## 第二轮：代码质量检查

- DB mutation 参数化；安全 error 枚举、方法白名单及路由模板长度约束，不存正文、原始 URL、query、Authorization、Cookie、IP 或 UA。
- 每次写入与查询限时 3 秒；响应开始前存储失败只拒绝该请求；不改全局生产配置。已开始输出后无法改写 HTTP 状态，已有证据继续保留。
- 实例 lease 由 PG 时间决定。新一代只接受新请求，过期 WS/worker 父实例不能被新 turn 复活；重复终态 first-write-wins。
- attempt 序号在行锁事务内分配；output 标记每 attempt 一次；通用 SSE metadata 缓冲最多 64 KiB/行且不持久化正文，已接入的业务 parser 以权威终态覆盖该保守 fallback，不以缓冲上限否定已确认的大终态。
- billing 验证和原 monetary/dedup effects 在同一事务中；验证失败回滚收费。异步任务 wallet reconciliation 只读现有 capture/release 证明，hold 不等于 settled。
- 后台 recovery 和 task reconciliation 每轮最多 1000 条，按检查时间轮转，避免一直卡在旧待核对项；无自动删除。
- 只读 API 请求列表分页有上限、排序稳定；详情 attempts/billing 子集合未分页，详情及 usage 原记录二次验证 owner；普通用户隐藏调度/母账号 ID。
- 新表/索引 additive、forward-only，锁等待 2 秒、语句 30 秒；不扫描修改旧业务大表，无旧迁移 checksum 改动。
- 前端沿用共享布局、表格、过滤器、分页和对话框；刷新/取消请求使用 AbortController 防止旧响应覆盖新筛选。

兼容修复：更新 cleanup 测试构造参数；cleanup 对 legacy nil fixture 兼容；恢复原 WS nil-connection 错误语义。

## 验收矩阵与证据层级

| 场景 | 自动化证据 | 限制 |
| --- | --- | --- |
| 401/429 与全部注册网关路由 | actual Gin 注册表 + PG；另有实际 API-key middleware 无凭据 401 | 429 矩阵注入拒绝，不冒充所有实际限额分支 |
| 400/500/504 | HTTP handler + PG；实际 body limit 400 | fake handler 状态用于台账链路验收 |
| HTTP/SSE 超时、部分输出、断流、取消、重试 | fake upstream HTTP transport + PG | 无付费外网调用 |
| 两轮 Responses WS | actual incoming coder WS + 原 Proxy service + fake upstream + PG；合并 PR346 后重验缺失 response ID 的终态 | 不等于所有 WS 协议分支完成端到端验收 |
| WS 已连接后 1013/EOF，无 usage | 入站与上游均为实际 coder WS；原 Proxy 的两次发送均先有 PG user/key/account/母账号及 attempt | 完全合成身份/事件；未知用量不产生结算引用，重复恢复不重放，不使用生产请求标识 |
| 进程被杀、重启恢复 | 真子进程/上游收到发送/kill/新 Ledger/PG | 加速 fixture lease 过期，不等待生产 90 秒 |
| settlement 失败与重复终态 | 真实既有 billing repo + wallet + PG/Redis，事务约束故障注入 | 未改余额算法，无生产收费 |
| 角色、分页、刷新、usage 原记录 | 真实台账入口/Transport → fake HTTP upstream 检查先有持久证据 → 现有 billing repo/usage writer → 真实 API + 全迁移 PG | 鉴权为合成身份，协议解析为测试 adapter，不冒充完整生成 handler |
| 保留 | 删除 fixture 旧 usage 后 ledger/settlement 仍在、引用变 NULL | 只测试隔离 PG，不删除生产记录 |
| 图片批次/移动视频/任务重放 | fake HTTP、真实 worker/provider 路径与 PG；wallet capture 真实 repository | 未涵盖所有第三方任务完整生命周期 |
| Realtime/Live 自动 turn | PG 状态机及既有协议测试 | 完整模式端到端矩阵未完成，见规格边界 |
| UI | production assets + API/PG 的合成身份浏览器 | 不代替用户本地生产三身份验收 |

## 首批 83c8dcb 门禁（不替代独立审查修订门禁）

| 命令/检查 | 结果 | 证据与边界 |
| --- | --- | --- |
| `make test-backend-unit`（合并后） | PASS，2026-10-10 06:21:54 UTC，exit 0 | 完整 tag=unit，含 PR346 与最终台账归因修正；低负载串行 |
| `wire` | PASS，05:37:49 UTC，exit 0 | 重新生成服务 DI |
| `go test -tags=integration ./internal/requestledger/acceptance -run '^TestRequestLedgerHTTP' -count=1 -timeout=3m -v` | PASS，05:39:19 UTC，exit 0，14.224s | 新 HTTP→实际 Transport→fake 上游→既有结算→usage/API；角色、分页、保留 |
| `make test` 首次低负载完整尝试 | FAIL，05:49:35 UTC，exit 2 | 普通后端全通过；lint 的 1 个 errcheck、2 个静态简化项已修正；不是退出 137 |
| 合并后 PG 定向套件（命令见下） | PASS，06:13:00 UTC，exit 0 | 核心 61.692s、API 15.451s、repository 8.386s、service 26.145s |
| `make test`（合并后最终） | PASS，06:37:31 UTC，exit 0 | 完整 Go 测试、golangci-lint 0 issues、前端 lint/类型/设计治理；492 文件、3497 测试通过，1 文件/2 测试按既有定义跳过 |
| `go test -tags=integration ./internal/service -run '^TestRequestLedgerRealWebsocket1013AndEOFWithoutUsage$' -count=1 -timeout=3m -v` | PASS，06:38:46 UTC，exit 0，5.938s | 两个真实网络子用例；既有 retry 不改策略，未知费用不估算；CI 明确要求该测试执行通过 |
| `make build`（合并后最终） | PASS，06:42:19 UTC，exit 0 | 后端 CGO=0/trimpath 生产二进制、前端语言完整性与生产构建 |
| `./scripts/check-fork-integrity.sh` | PASS，06:44:57 UTC，exit 0 | 完整静态、文档及受保护前后端行为检查 |
| 最终 production assets 浏览器 + PG/API | PASS，06:45:40 / 06:45:42 / 06:46:11 UTC，exit 0 | 两角色刷新/四视口/浅深色/键盘/空态/英语/分页/加载/错误重试；原 usage 7/3 token、已扣 $1.25、同 PG 余额 $98.75 |

最终浏览器 fixture 于 06:46:21 UTC 正常退出 0；只停止本任务测试服务。余额断言首次遇到三个相同文本节点的 strict-selector 歧义（06:45:45 exit 1），限定为实际页头余额后精确比对 $98.75 通过；未改产品代码或金额期望。最终截图已目视检查，服务器证据不替代生产本地验收。

上述门禁对应首批 head 83c8dcb。独立审查发现新的可复现缺陷后已修改后端；原门禁与 CI 不能证明这些修改通过。最终集中修订需重新执行必要门禁。

### 主线组合与独立审查修订

首批草稿 PR #351 head `83c8dcb337db16fd9cc3ffb406dd327853d8642a` 的四个 GitHub workflows 全部成功：
Security 38032181543、Ledger 38032181561、Core Migration 38032181519、Fork 38032181523。没有取消或重跑 CI。
该 head 的独立审查存在阻塞，不能合并。

曾以 `--no-commit` 组合 PR349 `de3155f`，其 backend tree 与 83c 相同，前端测试于 07:02:56 UTC、构建于 07:06:12 UTC 退出 0。
随后独立审查修复改变后端，且根线程授权了含 PR350 的主线 `63eabcb6a5384f64aaf26f043e73818b8220cbb1`；最终一次合入该主线后不能再复用“后端树完全一致”的理由。

| 审查缺陷 | RED 证据 | 修订验证 |
| --- | --- | --- |
| 健康实例终态写失败长期 inflight | 07:13:46 批次失败 | 07:16:26 GREEN；本实例活跃集合保护真正活动请求，失败终态可回收，其他健康实例保持 inflight |
| POST 伪 Upgrade 变成非计量 WS | 同上实际 HTTP/SSE | 同上 GREEN；GET 与实际 WS 路由共同判断 |
| ObserveUsage 注记失败吞掉原有结算 | 07:20:32 OpenAI/Anthropic 行锁故障均失败 | 07:29:12 GREEN；实际 dedup/usage/wallet 正常，重复不扣款 |
| 失败零快照被显示为已结算零金额 | 07:24:23 实际 usage promotion 故障失败 | 07:29:12 GREEN；link 与 usage 均验证才显示金额 |
| 提前关闭 / bare error 后成功 / 大终态帧 | 07:23:08 实际业务 parser 回归失败；07:36:00 补充 Chat/Messages 大终态仍失败 | 07:45:30 最终矩阵GREEN，native/CC及Chat/Messages转换均复用业务终态元数据 |
| 压缩 SSE / OAuth models 直连 / 图像回填归因 | 07:32:23 四编码全部误判、models 缺少发送前证据、图像回填归因/可忽略失败错误聚合 | 07:36:00 直连/四编码/回填GREEN；实际models缓存路径另于07:58:59 GREEN，见后文 |

锁顺序是并发审查的加固候选；未证实稳定死锁，不把它写成已复现根因。迁移为五张新表；详情子集合未分页、lease 单行热点及匿名增长均保留为容量限制。局部并发正确性探测不代表生产吞吐验证。

```bash
go test -tags=integration ./internal/requestledger/... ./internal/repository ./internal/service \
  -run 'RequestLedger|LedgerPostgres|LedgerKilledProcess|WSObservedUsage' -count=1 -timeout=12m
```

## 验证环境记录

本次容器 `/tmp` 仅 8.8 GiB，曾被任务 Go cache 用满；受影响命令按失败处理，未提交。
后续缓存与编译目录迁至 `/workspace/ledger-build-cache` / `/workspace/ledger-go-tmp`，只清理本任务的可再生成旧缓存。
曾出现 xxhash CGO=0/trimpath 的 ABI 链接错误：不含项目代码的最小程序同样失败；仅使该缓存 archive 失效重建后最小程序退出 0，未改依赖或业务代码。
默认 lint cache 指向只读 home 导致门禁失败；后续显式 `GOLANGCI_LINT_CACHE=/workspace/ledger-lint-cache`，不跳过 lint。
一次并发运行的 lint 退出 137；同轮完整 unit 的 `TestOpenAIWSHTTPBridgeAcceptsFirstFrameAboveLegacy16MiB` 与 `TestOpenAIWSHTTPBridge_IdleTimeoutClosesClientSession` 超时。未修改断言或超时：两项隔离复验退出 0（2.781s），低负载完整 unit 也退出 0。未修改基线 `3843ff3` 同两项 `-count=1` 对照于 05:37:19 UTC 退出 0，未复现。三组结果均无断言/超时改动；未重建当时的并发高负载条件，因此不把最初失败正式归因为环境。
最终重型命令串行，`GOFLAGS=-p=2 GOMAXPROCS=2 GOMEMLIMIT=7GiB`。前期使用 `GOGC=50`；确认容器上限 16 GiB、单一重型进程后，后续门禁恢复默认 `GOGC=100`，不改变测试断言或超时。工作区剩余空间降低后，后续 `GOTMPDIR=/tmp/ledger-final-go-tmp`，缓存仍在工作区；不删除源码或证据。

## 发布边界

没有生产写入、付费测试、停服务、凭据签发、生产配置变更、合并或部署。
PR346 已按根线程授权合入；保留它的 claim、快照与结算算法，本分支组合私有台账 context。组合测试单独列证据，不能继承其全绿结论。
全介质故障、应用接纳前拒绝、WebRTC 直连媒体及插件内部重试不在本台账可无限保证的范围内。
匿名流量持久保留会增加容量和写入压力；容量/归档治理另案，不擅自加清理策略。

## 集中修订补充证据

- 07:33:04 UTC：迟到 usage 的精确 attempt 回归 RED；07:36:00 批次中该项 GREEN。handler 入队即冻结 attempt；SQL 不再选“同账号最新一条”。WS 原精确回调保持不变。
- 07:36:00 同批局部并发探测：12 workers、48 requests、Recover 并行，558.863ms，全48条 succeeded/known；两次均通过。只覆盖本地有界负载，未做生产容量基准。
- 辅助 GET/HEAD 新 phase 为 auxiliary，不替代模型 attempt，也不把可忽略资源失败覆盖模型结果；详情保留该发送失败，中文/英文沿用现有阶段列。
- 台账注记失败日志使用固定枚举，不包含错误正文；原扣费继续，无法证实的用量仍保持未知，不能用已结算引用反推总上游费用。
- 07:35 UTC 左右任务工作区剩余空间降至约562MiB，仅删除本任务 Go cache 内两小时前的可再生条目，释放约5.1GB；未删除源码、PG持久数据或验收证据。

- 07:40:57 UTC：第一轮集中全矩阵退出0，六个包全部通过（核心70.852s、API14.216s、handler5.960s、路由11.741s、repository18.198s、service38.265s），含转换 Chat/Messages 大终态、全部实际路由拒绝、WS及原有结算。
- 07:41:07 UTC：最终边界检查 RED，HEAD/204/304 无正文被判失败、nil父上下文触发 panic；按 HTTP 语义与原调用兼容性修正，未放宽异常流断言。随后受影响矩阵复验。
- 原始与新门禁日志均留在本任务证据中；CI需30项明确pass，旧head的4个workflows全绿不能替代最终head。

- 07:45:30 UTC：最终受影响矩阵全部GREEN（核心75.231s、API14.669s、repository16.183s、handler7.481s、service48.101s）。路由注册未改，复用07:40:57矩阵的实际注册路由证据。30项CI必需contract均实际执行pass，无隐藏跳过。
- 本修订沿既有 parent→attempt 顺序加固 usage/output 注记锁序；只写为并发加固，不声称已复现生产死锁。
- migration275是本草稿首次引入且尚未发布的迁移，因此 auxiliary 枚举在同一新迁移定义；未更改任何已发布旧迁移。

- 07:53:40 UTC：`make test-backend-unit` 完整退出0。随后复核发现 models 共享缓存实际刷新使用 Background，上述直连回归不足以证明实际 cache path 已归因；补充缓存未命中/过期异步刷新与存储门禁回归，不把直连GREEN冒充该入口已覆盖。

- 07:55:24 UTC：真实 `fetchCachedOpenAIModels` 未命中和过期后台刷新均RED，上游检查到0条持久attempt；07:56:07 子记录存储故障用例RED，仍发出上游。修订仅触及既有models审查项：`refreshCachedOpenAIModels` 复制首触发者的已验证handle，先建非计费子执行；实际HTTP发送仍走既有attempt门。命中/合并请求不伪造额外发送。

- 07:58:59 UTC：models 实际缓存路径GREEN，含miss/stale、原HTTP先结束后刷新仍独立完成、缓存命中不多发、跨用户不可读取该子记录、子记录存储故障拒绝发送。定向同时复验直连models、健康实例恢复与HEAD/204/304；service15.259s、ledger13.651s。
- 该models修订后再次冻结代码；完整unit及其后make test/build/Fork最终结果另列，不沿用前次unit作为最终修订通过证明。CI必需项增加至32项。


## 冻结修订最终门禁

源码校验清单建立于08:01:45 UTC，覆盖本轮36个变更源码/CI文件。08:20静态检查后仅对故意nil的测试断言增加局部注释；再按根线程授权于安全点合入a581d8d，并新增JSON/SSE组合回归。最终源码树重新核验，不能称08:01之后完全没有修改。

| 门禁 | 最终修订结果 |
| --- | --- |
| `make test-backend-unit` | 08:09:13 UTC，exit0，包含最后models缓存修订 |

| `make test`（旧63e组合） | 08:20:34 UTC，exit2；普通Go测试全部通过，lint唯一SA1012来自故意nil-context断言。保留断言并增加局部nolint说明；前端/build未启动，不计通过 |

### PR348 主线安全点组合

- 08:22:12 UTC：真实PG台账中间件+实际handler回归RED，已提交400 JSON后错误追加SSE；旧63e组合只用来证实回归。
- 原门禁正常结束后，将未提交的63e组合替换为根线程授权的a581d8d普通merge；自己的26个tracked修改及6个新文件先做完整备份，三方应用无冲突。没有rebase、force或改共享工作树。
- 合并前后Git子树核验：frontend `f393d93000233cfeb8dbbc61e4bdf072c4319632`、requestledger `9adf2953f66cbd75a6d37601f74d8f6a43c6f7b6`、migrations `2c4fe9a42d465811c7f426b5f48068f8708ffd36`完全相同。repository唯一差异为主线新增`openai_compact_admission_integration_test.go`；其生产源码及台账测试不变。
- 共享handler/service重验实际PG、WS、JSON单响应、usage归属及完整tag=unit；repository复验两项主线真实PG admission及台账结算。未改变预提交attempt、observed usage禁止重放或主线选路策略。
- 复用不变核心/API/路由的07:40/07:45实际PG矩阵与08:09完整unit证据；完整make test/build/Fork及前端刷新仍需在最后组合执行完成。

- 08:26:31 UTC：新主线组合的真实JSON/SSE回归GREEN（8.22s），整handler集成包于08:26:35退出0（14.413s）。CI必须执行并pass此案例，总数33项。

- 08:28:13 UTC：最新a581组合的handler/service PG交集矩阵退出0；handler14.413s、service55.476s。包括实际WS、重试/断流、业务流终态、models真实缓存刷新和图片回填。

- 08:29:21 UTC：repository实际PG矩阵退出0，覆盖最新compact/tool admission与台账结算/观察失败/去重/压缩路径。

- 2026-10-10T08:32:32.899637+00:00：仅清理本任务一小时前未使用的可再生成Go缓存，释放1235772966字节；未删除源码、数据库或证据，活动门禁没有取消。

- 08:36:15 UTC：最新组合的完整handler/service tag=unit退出0（53.347s/216.307s）；未改变断言或超时。随后串行启动完整make test/build/Fork。

- 2026-10-10T08:42:00.875221+00:00：为后续构建清理合并主线前生成的8个大型可再生编译cache条目，释放1840844016字节；源码、证据和PG数据未动。完整普通Go测试已通过，继续原lint命令。

- 08:51:46 UTC：最终a581组合`make test`退出0；普通Go全通过、golangci-lint 0 issues、前端设计治理/ESLint/类型与完整测试通过。构建随后开始，没有并行重型门禁。

- 08:57:51 UTC：最终组合`make build`退出0，CGO=0/trimpath后端生产二进制、前端语言完整性及Vite生产构建全部完成；随后启动完整Fork检查。最终make test前端为492文件/3506测试pass，仓库既有1文件/2测试skip。

- 09:00:58 UTC：完整`./scripts/check-fork-integrity.sh`退出0，包含静态/文档及受保护前后端行为。随后才启动最终生产资源+隔离PG浏览器验收。

- 09:01:27 / 09:01:29 / 09:01:36 UTC：最终production assets+新隔离PG的三项浏览器脚本全部退出0；两角色无pageerror，四档视口与刷新、辅助下载、英文、分页、加载/503重试、键盘/空态、原usage7/3、已结算1.25及同库余额98.75均通过。测试服务09:01:36退出0，没有停止生产服务。代表性最终截图已目视核验。

## 首次集中修订复核结论（13db，已被第二轮阻塞）

执行线程按规格与质量两轮复核了根线程独立审查所列问题：每项修复保留RED与实际PG/业务路径GREEN；全部33项CI必需契约均明确执行通过。最后组合的完整make test（lint 0 issues、前端3506 pass）、make build、Fork与真实API浏览器门禁通过。108个源码/CI文件冻结后未变更，最终提交前再次核对；最后提交的GitHub CI及根线程独立组合复核仍以PR为准，不冒充外部批准。

证据索引：[`2026-10-10-request-ledger-evidence/`](./2026-10-10-request-ledger-evidence/)。摘要仅含命令、时间、退出码、测试名称、源码哈希及合成身份浏览器结果，不提交原始请求、正文、凭据或生产数据。已知能力/容量与批次drilldown边界仍按规格保留。


## 第二轮独立审查修订（13db 后续）

已发布 `13dbf1a4289b305a42873bf11bb1d3854910c3bd` 的四个 workflows 全绿（Security 38040011337、Core Migration 38040011329、Ledger 38040011450、Fork 38040011327），但独立审查发现以下五组阻塞。旧 CI 不能证明修订通过，PR 保持 draft，由根线程对新 SHA 增量复审。

| 阻塞 | RED：实际业务路径 | 修订与 GREEN |
| --- | --- | --- |
| Live 已收到 frame 的审计注记失败被当成 transport read error，丢失原 Redis/结算用量 | 09:20:58，真实 WS + PostgreSQL constraint 故障：Redis 0、原 outbox 快照 0、余额未扣、charge 0 | ReadFrame 保留真实 frame，固定安全日志；Observe 完成清理后返回注记错误。09:28:12 实际 Redis 7/3、原 PG outbox 快照 7/3、wallet 100→99.87、usage 0.13；重复 done/finalize 仅一笔 charge，只有一次连接 |
| Live async_execution child 的 attempt 终态失败使健康 owner 永久保护已退出 turn | 09:17:29，实际 Observe 与 Close 两条路径均保留 inflight | attempt 与 turn 终态均尝试，async_execution 结束释放 child liveness；同 owner Recover 后 interrupted/usage_unknown |
| WS→HTTP bridge 与 OAuth images 缺少业务 parser 终态回调 | 09:17:29，bridge bare error→completed、>128KiB completed；真实 forwardOpenAIImagesOAuth Responses backend 非流/流 completed 均失败 | 在既有业务 parser 获取 type/status，关联精确 attempt；流提前返回及非流 EOF 的真实路径均保存 succeeded |
| native/passthrough nil error 被误作成功，覆盖 incomplete/done failed/cancelled | 09:17:29，两路径共 10 个非成功变体失败，2 个成功对照通过 | 使用实际 event type 与 response.status；[DONE] 仅后备，不覆盖语义终态；所有 12 个变体通过 |
| models 插件拿到 credential account 后覆盖 scheduled ID | 09:17:29，实际 PluginManager/runtime/双向协议发送点 PG 中 scheduled=22，期望234 | 新 BeginRoutedAttempt 仅恢复 context 内已验证 scheduled identity；插件仍收到母账号22，发送前 PG 为234/22；不改变路由、rollout或headers |

09:25:29 的首次组合 GREEN 命令退出1：bridge/images 与实际 Live 结算通过；native/passthrough、Live child、plugin、healthy-owner 的 PG fixture Ping 在30秒未就绪，业务断言没有运行，不能算通过。清理本任务一小时前的可再生 Go cache（2,632,422,457 bytes，3952 files）后为 fixture 增加容器末40行诊断，未改超时或断言；没有留存首次失败的容器诊断，不能认定磁盘就是根因。

09:26:23–09:28:12，`review2-green-complete` 退出0。五组新回归与原 healthy-owner/models cache 回归均明确通过。09:31:43 冻结最终源码并启动全台账矩阵；冻结前补充的 nil context 兼容保护也已包含在清单内，最终门禁以本节后续结果为准。

### 规格复审

- 发送前入口/attempt 持久门仍同步且失败拒绝当前发送；只有已经收到上游 frame 之后的审计注记故障允许原业务使用真实数据，不能触发重放。
- 终态回调传递业务已解析的类型/状态，不存正文、不重新估算用量。执行/用量/结算三轴独立；Live 审计未知不会伪造成零费用，父级原结算可被核对。
- 每次结束都释放已退出处理的 liveness；保留健康工作保护和跨 owner 租约边界。恢复不补发、不补扣。
- 插件依然接收原 credential account，台账独立保留 scheduled/mother，未扩大插件内部可观测范围。
- 本轮无迁移、API查询、UI或收费策略变化；不把 Responses image 回归声称为所有图像供应商或所有 Live 模式的完整端到端矩阵。`openai_images.go` 的 direct-image `image_generation.*` 专用流（尤其无[DONE]）没有新增真实终态回归，不能用本次四项bridge/Responses-image通过为其背书。

### 代码质量复审

检查了 nil receiver/context 兼容、defer 与 body close 顺序、锁内终态/清理、固定日志枚举、实际 terminal status 映射及插件 identity 匹配条件。真实结算测试通过仅 test 编译的导出 fixture 注入原 PG repository，避免生产 service→repository 导入及生产测试开关；外部测试包使用一次性 Redis/PG。新五项均列入 CI 强制执行，要求38项明确 pass，跳过不能代替。

未修改既有收费、dedup、快照、订阅或wallet算法。新的完整门禁与最终源码哈希归档到相邻 evidence 目录；原RED、fixture失败和旧门禁仍保留为历史，不混作新 head 的通过证明。

- 09:35:11 UTC：最终 PG 全矩阵命令退出1。核心82.940s、API15.610s、handler8.395s、routes13.810s、repository25.072s均明确通过；service编译因缺少缓存archive而失败。09:34:46清理本任务大型旧archive时，误删了运行中compiler仍引用的ent/redis文件，日志给出精确缺失路径，属于执行线程缓存维护错误；不是业务测试通过，也不能归因应用。所有后续门禁链已自动停止。保留失败记录，不改源码/断言，停止编译后重新构建受影响service。后续禁止在运行中的编译期间清理缓存。

- 09:39:07 UTC：受影响 service 完整台账集成矩阵重建后退出0，实际84.687s。最终冻结源码的38/38必需契约全部明确pass；五个先前通过包与本次service组合构成完整矩阵，原整条命令exit1仍保留，不伪改。09:39:09开始完整tag=unit门禁，随后串行make test/build/Fork及最终生产资源浏览器。

- 09:47:08 UTC：完整 `make test-backend-unit` 退出0；随后在无编译进程的门禁间隙清理本任务可再生大型cache（1,465,493,174 bytes / 18 files），09:47:09开始 `make test`。后续空间维护仅在同步门禁结束后的安全边界运行，记录在执行环境 `ledger-review2-boundary-cache.jsonl`。

- 10:03:43 UTC：完整 `make test` 退出0，普通后端全部通过、golangci-lint 0 issues、前端设计治理/ESLint/类型通过；前端492文件/3506 tests pass，既有1文件/2 tests skip。10:03:44开始生产构建；之前仅在空闲门禁边界清理11个大型可再生cache条目（837,654,210 bytes）。

- 10:07:46 UTC：完整 `make build` 退出0，后端CGO=0/trimpath发布二进制与前端生产构建通过。10:07:47开始完整Fork检查；空闲边界清理6个可再生大型cache条目（466,836,486 bytes），未在编译中清理。

- 10:16:57 UTC：完整 `./scripts/check-fork-integrity.sh` 退出0，文档、静态及受保护前后端行为全部通过。10:17:00开始最后生产资源+隔离PG浏览器fixture；此前安全边界清理19个可再生大型cache条目（1,464,570,896 bytes），没有并行编译。

代表性最终截图已目视核验：管理员英文usage/母账号详情、普通用户360px列表、管理员深色列表；没有将旧截图当作本次通过证明。

### 第二轮最终冻结门禁

| 门禁 | 结果 | 实际命令 |
| --- | --- | --- |
| `review2-final-service-pg` | 09:39:07 UTC，exit0 | `go test -tags=integration ./internal/service -run RequestLedger|LedgerPostgres|LedgerKilledProcess|LedgerStorage -count=1 -timeout=12m -json` |
| `review2-final-unit` | 09:47:08 UTC，exit0 | `make test-backend-unit` |
| `review2-final-test` | 10:03:43 UTC，exit0 | `make test` |
| `review2-final-build` | 10:07:46 UTC，exit0 | `make build` |
| `review2-final-fork` | 10:16:57 UTC，exit0 | `./scripts/check-fork-integrity.sh` |
| `review2-browser-fixture` | 10:19:04 UTC，exit0 | `go test -tags=integration ./internal/requestledger/acceptance -run ^TestRequestLedgerBrowserFixture$ -count=1 -timeout=22m -v` |
| `review2-browser-responsive` | 10:18:53 UTC，exit0 | `node /tmp/ledger-visual/verify.cjs` |
| `review2-browser-states` | 10:18:56 UTC，exit0 | `node /tmp/ledger-visual/states.cjs` |
| `review2-browser-detail` | 10:19:03 UTC，exit0 | `node /tmp/ledger-visual/final-states.cjs` |

全部38项CI必需contract明确pass；113个源码/测试/CI文件与冻结清单一致。最终浏览器复验使用最后生产构建、隔离PG与合成身份；两角色刷新、权限、分页、主题/视口、真实原usage及余额对账均按脚本执行，不代替用户本地生产验收。PR新SHA的GitHub CI及根线程增量独立复审仍以PR记录为准。
