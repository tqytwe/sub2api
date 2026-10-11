# HTTP 文本路由证据后端小批

状态：add3 CI既有worker测试采样竞态已用6行有界等待修正；受控RED/GREEN、永久丢Ack负向、race、按序双审与全部本地门禁通过。新head CI及主线程复核待完成，未合并部署。
维护责任：本任务提交者；生产发布由主线程协调。

基线 `23d06337cbc315cb92d3fc18e94902fe21e7342d`；独立分支
`codex/routing-evidence-20261011`。发布前已普通 fast-forward 合入
`05b44bec5c2d30ad58d009fbe5247ea363c6ee70`，保留 #352 全部修复。随后按主线程通知普通
merge `1b701237ec3ceb869c39d28ad11d32e4065a2471`（#358），认证/Quiz改动原字节保留；
只有文档索引并列条目冲突，保留双方条目。无 rebase/force。

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
迁移在改表前显式取得父表 `ACCESS EXCLUSIVE NOWAIT`；读锁或活跃写事务争用时立即失败，
不排队到正常 Begin/Finish 前面。其他锁等待上限50ms，单语句执行上限500ms，两列在同一
ALTER 中增加。失败由既有运行器回滚整个迁移事务（包括迁移登记），解除争用后重新部署重试。
保留旧应用处理流量，不要求停服或人为清空流量窗口，也不自动循环抢锁。

NOWAIT 不能消除实际取得锁后的元数据 DDL、提交、调度及存储 I/O 开销；500ms是 PostgreSQL
单语句时限而非端到端零阻塞保证。取得锁后的短暂排队仍可能发生，异常 I/O/取消响应也不属于
硬实时保证。发布前看到10933行/9.35MiB及无长事务仅是主线程当时快照，不保证未来无争用。
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
CI 由独立草稿 PR 触发，不合并或部署。原head8665a7af的9项CI均通过；主线程随后发现
279发布锁风险，明确要求此增量修复并对新head重新执行CI，旧head不得合并。

### 原实现本地门禁

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

原实现技术验证提交：`0702e5bf41018a1aef8b8f9ff4fcb931ddfc4160`；原元数据head为
`8665a7af963e4b2ff4d7537cdb1e9c19e3b38580`，已被迁移锁修正取代，禁止按旧head发布。
增量只修改279、迁移回归测试、CI必需用例及交接文档；运行时路由和证据代码保持原字节。
当时分页PR360的278尚未合入，未创建占位迁移；后续已按主线程通知合入主线#360，见末尾组合记录。

### 279 发布锁修正的 TDD

真实PG经实际迁移运行器复现：读事务和活跃写事务各使旧迁移排队约5.02秒，普通旧版
Begin/Finish各自在3秒截止时被取消；延迟CREATE两秒时旧迁移仍成功，证明缺少短执行时限。
新增测试检查读写争用立即55P03，旧SQL继续准入/结束，失败后两列/子表/迁移登记无残留；
延迟CREATE触发57014并回滚此前已增加的两列，普通请求随后在原3秒内完成；解除争用后
真实运行器重试登记正确checksum，历史行保留，新旧writer均可Begin/Finish。
校验和与最终结果见validation.json；旧275–277未改。279尚未部署，此次按主线程明确授权
修改未发布迁移；发布后必须保持其校验和不可变，不把草稿旧checksum加入兼容白名单。

### 279 + #358 组合最终本地门禁

基线1b701237；增量规格/质量及合并保留复审均通过。前端与新主线字节相同，其他
handoff条目完整保留。真实PG迁移回归带race连续3轮通过；台账66/66必需项通过，
核心迁移/仓库54/54必需项及203测试/子测试通过。台账首轮因PG initdb磁盘不足失败，
完整重跑受影响service后通过；其他已通过包的后端源码未变，原失败单独保留。

make test（后端全量、lint零问题、前端lint/typecheck、500文件/3644测试通过、既有
3文件/8测试跳过）、make test-backend-unit、make build、embed build、完整Fork检查、
文档链接、8项交接测试和26文件源码校验清单全部通过。
本次只清理可再生成的旧Go缓存及明确观察到的已退出临时ledger容器卷，未知既有卷未动；
未手工终止任何测试，未改断言以规避环境失败。

279文件SHA256：`48abb66e1d91478513d87973e750d802a330cf15cde4fd9764b3866d2ea96fd4`。
运行器trim后登记checksum：`4db70ab30cae713b86f653010ee4f5c0ffe3fcddf1ba151a4bbe15b078dac715`。
新head仍需独立CI及主线程增量复审，之后由主线程决定合并部署；本任务不发布。

增量技术验证提交：`eb8cbab6786df2a2e880b3f95afc7b21669c3b4f`；后继提交仅补交接元数据，26文件源码清单不变。

### 合入 #361 冻结计费绑定修复

普通无冲突merge `fdec971d2a01c25d93874b7688dffcff78f79088`，父分支原head为
`ebcba25f01775d226d93f0a1031398390739e2fe`。26个路由/279/CI输入与旧head逐字节相同，
55个非共享主线文件完整保留，两个共享索引保留双方条目，前端及迁移零变化。
本任务没有另改计费逻辑；继承主线billing.go冻结subscription/entitlement绑定校验。
保留证明见[merge-361-proof.json](./merge-361-proof.json)，依次通过规格、质量复审，
未发现Finish字段覆盖或异步worker互锁风险，见[merge-361-reviews.json](./merge-361-reviews.json)。

三套计费HTTP/PG合同分别使用新建数据库执行含279的全部迁移，均通过race检查：
套餐/订阅计数42个叶用例、绑定问题复现4个、绑定兼容36个。
路由/Finish/计费组合回归、完整后端普通测试与lint、tag-unit全量、make build、
embed构建、Fork完整性和文档检查通过。具体命令、耗时、原始日志摘要哈希及
白名单测试事件见[merge-361-validation.json](./merge-361-validation.json)和
[merge-361-test-results.json](./merge-361-test-results.json)。

按主线程明确要求复用无增量的长测试：前端源码完全相同，沿用ebcba25f的
lint/typecheck及500文件/3644测试通过结果（保留既有3文件/8测试跳过）。
此次执行make test-backend，结合上述前端结果覆盖make test的两部分；未声称重新运行
整个make test。旧66项广协议与54项核心合同只保留历史范围证据，billing.go已变，
不能称整个集成套件同源码或新head本地重跑66项。

279原始及运行器checksum不变，未引入分页278。新head CI与主线程增量复核待完成；
UI接线、合并部署和用户本机生产验收仍由主线程协调，本任务未访问生产。

#361组合技术合并提交：`dec21d65da0540bead469f95713feb7fab829ca8`；后继只补交接元数据，26文件源码清单不变。

### 合入 #360 分页与会话隔离

普通merge `fb2c9747b58ed5d980a36ea6a2316ead2aa03b7e`，此前head为
`ae832b16b63139b2395008a1f2073d7cb15161f7`。24个非共享routing输入及279原字节保留，
95个非共享主线文件完整保留；共享fixture合并279初始化与主线卷清理，CI并集增加一项
278→279实际runner组合回归，共75项必需测试。无280/281文件，无API改动、rebase或force。

本次实际HTTP/PG检查覆盖分页、独立routing reader及权限/故障隔离、Finish/worker/锁/池/
崩溃，并以实际迁移runner执行278并发索引后279，确认checksum、有效索引、历史保留、
幂等重启和新旧writer。首轮4个迁移夹具在PG启动阶段退出，尚未执行断言；容器自动删除
导致日志缺失，不能断言失败根因。保留首轮失败，移走可再生成缓存后完整受影响集合通过，
其他已通过包源码未变，无放宽断言或手工停止测试。

完整后端普通测试/lint、tag-unit、make build、embed、Fork及文档检查通过。前端与主线
源码逐字相同，按主线程要求复用主线3680测试通过/8既有跳过、lint/typecheck、26项HTTP
会话及浏览器证据；本次实际构建356资源和入口文件hash逐一相同。没有重跑整套make test
或本地全75项的声明；旧广协议结果是历史范围证据。

逐项结果、初次失败及重跑事件见[merge-360-validation.json](./merge-360-validation.json)、
[merge-360-test-results.json](./merge-360-test-results.json)；保留证明与按序双审见
[merge-360-proof.json](./merge-360-proof.json)、[merge-360-reviews.json](./merge-360-reviews.json)。
实际构建比对见[merge-360-assets.json](./merge-360-assets.json)。当前27输入清单记录两处共享
文件及新增组合测试，原26输入清单另存tested-source-ae832b16.sha256。

UI已由主线程另派分页作者，现有GET合同保持稳定；本批未接线UI、访问生产或合并部署。

### 合入纯前端主线 #362

#360组合技术合并为 `75069704437c99783530015f13cf11ed2748bbf7`，未单独推送。随后主线程
通知普通合入 `6ae024e6b732b64b2ce6f7ba56c5be4e20b88bdb`；唯一README邻接冲突保留双方。
262个非共享传入文件逐字保留，非routing handoff与新主线相同；backend/迁移/CI/scripts
完全等750，前端完全等6ae，27输入清单不变。按明确指示复用本目录刚完成的后端/PG全量
门禁和#362已审前端3789通过/8既有跳过、293+4定向HTTP/组件及36原生认证检查。

本次重新执行make build、embed build、Fork静态、文档与交接检查，实际全部356资源
逐一等#362已审构建。Fork静态检查来自原脚本，只将run_check运行分组标记REUSED，
全部静态断言仍执行；完整Fork结果来自750且后端/scripts无变化，前端运行检查复用
主线原字节证据，不宣称最新组合又执行了完整Fork或完整make test。前端原证据关于
auth.checkAuth、原生ledger/PG矩阵、永久Storage故障的边界继续保留。

结果见[merge-362-validation.json](./merge-362-validation.json)、
[merge-362-proof.json](./merge-362-proof.json)、[merge-362-assets.json](./merge-362-assets.json)、
[merge-362-reviews.json](./merge-362-reviews.json)。没有新增后端实现、API或迁移变动，
未触碰280/281；identifier保持独立暂停，routing UI由主线程另派作者。

#362最终技术合并提交：`c3f40d84280e454a0e0beedbab819b4e447a9b50`；后继仅补交接元数据，后端/前端与已验证输入不变。


### CI worker 队列归零采样修复

主线程报告 add3 head 的 Fork Integrity run38136192168 / job114460430956
在 tag-unit 的 `TestImageTaskWorkerRuntimeBoundsConcurrencyAndCompletesQueuedJobs`
最终 Active 断言读取到1。直接下载日志返回403，未独立取回日志正文，保留来源限制。

独立基线6ae工作树确认相关测试、worker运行时和Redis队列三文件与add3原字节相同；
未修改基线普通测试30次通过，没有自然复现。源码显示处理器completed计数先于
heartbeat结束和Redis租约Ack，Stats.Active统计的是租约ZCARD。测试专用overlay只
延迟最后两个Ack至首次Stats快照后放行，在基线连续3次触发原断言Active=2。
修复只增加3秒/20ms的Eventually等待Ready=0且Active=0，保留原并发上限、完成数、
第三任务启动限制与最终Ready/Active断言。当前Stats DTO没有Delayed字段。
运行时、队列Ack顺序、路由、迁移和结算代码未改，不称证明了生产worker缺陷。

修复后延迟Ack20次通过，普通100次通过；永久丢Ack的负向探针在3.18秒于新的有界
等待断言失败，证明真正不归零仍被检测。延迟Ack race10次、worker5个用例各5轮均通过，按序规格/质量审查均通过；结果
记录在[worker-ack-validation.json](./worker-ack-validation.json)。基线hash与测试事件
见[worker-ack-base-proof.json](./worker-ack-base-proof.json)、
[worker-ack-base-tests.json](./worker-ack-base-tests.json)，可复现的测试overlay生成器为
[worker-ack-probe.py](./worker-ack-probe.py)。探针只用于本地验证，不改生产代码或CI脚本。
原add3 CI失败保留；新head只普通推送一次触发CI，不盲目重跑原workflow。

本次重新完整执行make test（后端全量和lint0问题、前端504文件/3825测试通过，保留既有
3文件/8测试跳过）、make test-backend-unit、make build、embed build和完整Fork检查，
全部通过。没有用旧证据代替本次必需门禁。先前27个路由/279/CI输入原字节保留，
新清单增加worker测试共28项；历史HTTP/PG证据继续限定原范围，不称本地重跑全75项。
具体命令、耗时和原始日志哈希见worker-ack-validation.json，逐测试白名单事件见
[worker-ack-test-results.json](./worker-ack-test-results.json)，双审与保留证据见
[worker-ack-reviews.json](./worker-ack-reviews.json)、
[worker-ack-preservation.json](./worker-ack-preservation.json)。

本次测试同步技术提交：`0dcf99a3c5f786e41a8e2933dd025b1daea1c320`；后继仅记录交接元数据，28项验证输入不变。
