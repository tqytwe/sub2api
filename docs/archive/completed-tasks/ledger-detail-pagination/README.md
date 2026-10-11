# 请求台账详情有界分页

基线：`23d06337cbc315cb92d3fc18e94902fe21e7342d`（#356 merge）。
分支：`codex/request-ledger-detail-pagination-20261011`。
技术验证与独立审查在本记录收尾；本任务仅交草稿PR，不合并或部署，以下均非生产验收声明。

## 已核实现状与最小设计

原详情 GET 将 attempts、billing、wallet、Ops 关联全部读入内存；attempt
证据也单独读取整个请求。现有列表分页不会限制这些详情子集合。

- 原详情路径增加 `view=summary`，只返回 `request`。
- 同路径增加 `section=attempts|billing|wallet|diagnostics`，每个 section 独立
  `page_size`（默认20，上限100）与不透明 `cursor`。不计算无界总数。
- 每次读重新验证身份、管理员权限和请求归属；cursor 绑定请求、section、
  用户/角色，只是排序位置，不能代替授权。
- attempts 按 attempt_no；diagnostics 按 attempt_no/ops_error_log_id；
  billing 按来源及来源主键；wallet 按 task/operation 来源主键。
  尝试证据只读取当页编号，诊断继续是可降级的管理员信息。
- 先限制来源候选，再复用现有证明判断。被过滤的候选仍推进游标；允许
  `items=[]` 且还有下一页，UI 不自动无界追页，不把空中间页当读取完毕。
- 无参数旧 GET 仅在预算内能证明全部集合完整时返回原结构；否则返回
  `409 / PAGINATION_REQUIRED`，不提供不完整的成功结果。大量未认证 task
  也可能超过预算，不能只按最终可见条数判断旧接口是否安全。
- 前端继续使用既有 AppLayout、BaseDialog、DataTable 和按钮样式。每区域
  独立上/下页、刷新、忙碌、错误重试。切换请求或关闭时取消旧读取，迟到
  结果不能串到新请求。旧客户端遭遇409应刷新/升级至支持分页的页面。

主线程已确认实时分页契约：无状态实时seek保证稳定成员遍历不重不漏；
新插入、迟到usage/证明、关联成员可见性变化、字段更新均可能需要刷新。
API明确返回 `consistency=live` 与 `refresh_hint`，UI提供就地提示和独立刷新。
本批不做严格固定快照，不用高水位或顺序插入测试冒充；晚提交到旧位置必须测试刷新可见。

## 边界与验证

分页自身不修改计费写入、capture、usage认证条件或
`backend/internal/repository/usage_billing_repo.go`；组合逐字承接已审查的#352主线修复，
不手工改写该文件。不引入补扣、估算、TTL、
历史清理或猜测关联。历史 SQL/checksum 不变；新增278并发索引支持按request/task定位来源。
固定上游仍为 ranxi v2.10.3。

#352已记录的同用户/组下冻结entitlement-ID绑定缺口仍保留，分页没有修复该缺口，
也不以本地counter测试宣称任意entitlement替换或生产完整鉴权/调度/worker路径已获验证。
见[#352审查边界](../2026-10-10-subscription-counter-acceptance/2026-10-11-review.md)。

独立规格审查已通过，并复审timezone兼容与翻页期间usage禁用；代码质量审查也已通过；初始候选完整门禁通过，主线组合另验。TDD 首轮真实HTTP/一次性PG失败已复现：旧接口
忽略summary/section并返回105条尝试、无超界409、无参数校验/section权限。
RED 原始日志保存在执行环境 `/tmp/ledger-pagination-red.log`，后续归档摘要。

基线与DOM原型来自23d等价的实际构建资源及合成HTTP/PG fixture；原型先于
前端实现保存。不是生产浏览器验收，也不访问生产秘密或调用生成。

## 单独后续：安全只读补证

稳定工作项 `batch-image-readonly-evidence-recovery`：已完成任务在只读证明暂时
失败后，当前completed重试不会自动补齐审计认证。后续需独立设计精确
task/owner/key/capture冷热全部指纹核验、幂等补证与冲突拒绝，并证明不触发
capture、余额或订阅写入、上游生成、估算或历史猜测。本批仅登记，不实现。
该能力不能宣称恢复或归因历史gpt22损失。

初始候选全量门禁、HTTP/PG与真实资源交互及顺序规格/代码复审已通过；#352组合结果见文末，PR/SHA/CI独立记录。
本任务不合并或部署。

## 使用者/API迁移合同

普通用户使用 `/api/v1/requests/:id`，管理员使用
`/api/v1/admin/requests/:id`；每次读取仍经过既有登录与管理员路由权限。
API-key是请求归属与证明校验条件，不是此控制台API的登录凭据。

| 请求 | 返回/边界 |
| --- | --- |
| `GET .../:id?view=summary` | 标准envelope的 `data.request`；不读子集合 |
| `GET .../:id?section=attempts&page_size=20` | `data.items`、`page_size`、`has_more`、`next_cursor`；最多20条 |
| `section=billing` / `wallet` | 相同页结构；20是来源扫描预算，认证后可能少于20条 |
| `section=diagnostics` | 仅管理员，按精确attempt/Ops引用分页 |
| `GET .../:id?section=...&cursor=<上次next_cursor>` | 下一批候选；cursor必须原样使用，不能跨请求/用户/角色/集合复用 |
| 不带查询参数的旧GET | 仅在每集合100个认证候选预算内（加1条来源前瞻）确认完整才返回原结构；不能确认则409 `PAGINATION_REQUIRED`，无部分 `data` |

- 默认 `page_size=20`，有效范围1–100；不合法、重复或混用summary/section参数返回400。
- 兼容共享HTTP客户端自动附带的 `timezone` 传输提示：忽略其值，不改变绝对时间或分页。
  重复 `timezone` 和坏URL编码仍返回400；只有单个timezone的旧请求仍走legacy预算。
- `items`始终为数组。无可见证据的中间页可能 `items=[]`、`has_more=true`；
  只有空 `next_cursor` / `has_more=false` 表示本次实时遍历已到末尾，不能仅按数组空判断。
- 所有页明确 `consistency=live` 与
  `refresh_hint=new_records_late_evidence_and_state_changes`；它不是变更检测结果。
  固定成员集合按完整唯一来源键不漏不重。并发新行、晚提交、迟到usage/证明、
  关联成员或字段变化可能出现在已扫位置前方；刷新需丢弃该区域游标和旧显示页后
  以空cursor重读。不保存跨请求PG快照，也不承诺跨页同一时点。
- attempts稳定键为 `attempt_no`，证据仅按当页编号读；管理员才收到调度账号和
  上游标识。诊断稳定键为 `(attempt_no,ops_error_log_id)`，保留已过期原诊断的引用。
- billing先普通链接 `(billing_request_id,api_key_id)`，再batch task的 `task_ref`；
  顺序是来源顺序，不承诺跨来源时间排序。钱包按 `(task_kind,task_ref,operation)`，
  operation固定为hold、capture、release；按来源操作展示，不冒充按交易ID排序。
- cursor是有界的不透明位置数据，客户端应当只保存/回传。它不授予权限、不证明
  usage或扣款，也不是幂等键。SQL参数化；每页重新核对请求归属与原owner/key证明。
- 缺失或失败的attempt元数据用 `attempt_evidence_unavailable=true`；核心行继续
  返回。独立集合读取失败返回503；新UI就地错误与重试，其他区域保持可读。
  无权限为401/403，他人请求为404；不返回原始数据库错误。
- 新UI的详情方法显式发 `view=summary`，子区域只发 `section`；失败不回落到旧接口。
  老UI收到409需刷新到新资源或升级客户端。新UI遇到旧服务器的非分页形状显示
  读取错误，不当作完整页。滚动回退到旧后端不提供新分页功能，需配套旧资源。
- usage钻取路径及认证条件不变。分页只引用既有记录；null仍为未知，未认证金额
  不显示成0，共享batch结算不归给每次入口再次扣费。
- 所有详情/页响应保留 `Cache-Control: no-store`。无新缓存、快照表、TTL、清理或补证任务。

## 技术验证环境与证据口径

隔离worktree、Go 1.27.2、pnpm 9.15.9、一次性PostgreSQL 18.1 / Redis 8.4，
所有external DSN开关显式置空。HTTP由真实handler/router与PG执行，使用合成身份；
无上游生成、生产秘密、Zeabur、补扣或运营状态操作。实际Vite构建由opt-in测试fixture
服务，未启动 `go run serve` 或 `pnpm dev`。CI使用仓库自己的运行版本。

新增8个必需CI用例涵盖：API授权/参数/legacy预算、billing与wallet多页来源、
迟到capture/usage在已扫位置前需刷新、晚提交行刷新可见、分页证据隔离与故障降级、
并发attempt、22万task实际EXPLAIN有界索引，以及真实中断并发索引后的恢复/校验和/重复执行。
旧写入方不用新索引或字段；回退应用不需删除278。索引健康且登记一致才算迁移完成。

候选budget为每页1–100；多读1条只判断后续页，wallet每task至多展开3个固定操作。
没有为分页新增全量count、无限自动追页、跨请求PG快照或计费写入。

[实际资源浏览器证据](./browser-evidence.json)覆盖普通用户/管理员，360/768/1280/1920，
浅深色/中英文、键盘、reduced-motion及CSS200%模拟；legacy409、跨租户404、角色403，
所有新UI详情请求都显式指定summary/section，零pageerror。合成已结算usage为6美元/3图；
新页加载期间旧usage按钮禁用、既有明细清除。截图见[视觉记录](../../../visual-reviews/2026-10-11-ledger-detail-pagination.md)。

## TDD与已处理失败

- 首轮真实HTTP RED：原接口忽略summary/section并全量返回，未按预算409或验证参数；新增合同后GREEN。
- 并发索引恢复 RED：实际取消CREATE INDEX CONCURRENTLY留下invalid索引；278接入既有恢复后GREEN。
- 参数 RED：坏URL编码被静默忽略；显式ParseQuery后400。built浏览器发现共享客户端自动timezone
  被拒绝；新增真实HTTP RED→GREEN，单个timezone忽略，重复/坏编码仍400。
- 全协议首轮暴露旧合同回归：空attempt页面短路导致缺失可选证据表未给unavailable标志；保留有界空ANY
  查询后原case GREEN。后续资源耗尽导致PG容器启动失败，首轮被中止，不能当作通过。
- 前端usage并发 RED：翻页保留的旧按钮仍可点击；传递section loading并禁用后GREEN，迟到drill不会串页。
- 浏览器第一次脚本断言误用“图片数”，实际既有词为“图片张数”；修正脚本后重新完整运行，未改产品文案。

磁盘故障仅清理了本任务创建且Docker确认未引用的62个一次性匿名卷，以及Go闲置时早于本任务的
可再生成缓存；未清理生产或其他任务卷。测试fixture现在随精确容器清理其匿名卷。后续完整重跑结果
另列，不把资源失败隐藏或计入通过。

## 顺序独立审查

2026-10-11先由 `batch_spec_review` 只读规格复核，通过后由 `batch_code_review`
独立代码质量复核，两者均通过，无遗留实现阻断。规格复审覆盖timezone兼容与usage
翻页禁用的小修；代码审查覆盖来源LIMIT、wallet操作边界、授权/cursor隔离、可选
证据降级、取消/迟到UI响应、增量迁移恢复及历史/受限文件不变。两位审查者均未
重跑重型门禁；实际门禁由本任务执行，不能以审查结论代替测试或生产验收。

## 全协议重跑结果

2026-10-11 07:59:27–08:07:19 UTC全协议命令退出0，**56/56工作流必需用例通过，
94个顶层用例通过，无失败**。跳过项只有子进程入口、两个显式opt-in浏览器fixture及无测试的
helper包；本批浏览器fixture已另行实际执行。逐包耗时、精确required集合与命令见
[协议验证JSON](./protocol-validation.json)。[源码/构建指纹](./source-fingerprint.json)
绑定本轮技术测试与实际资源，之后纯交接元数据提交不冒充重跑源代码验证。

初始候选 `make test` 于2026-10-11 08:07:38 UTC开始、耗时877.14秒、退出0：
默认后端测试与golangci-lint、前端lint/typecheck及完整Vitest通过；500个文件、3611项
前端测试通过。既有opt-in的CheckInView.http（5）、QuizQuestView.pg（2）、BlindboxView.pg（1）
按未启用专用PG fixture跳过，共3文件/8项；分页组件19项和API3项均执行通过。

初始候选全部本地门禁已通过，见[逐项命令/耗时/摘要](./initial-validation.json)。
13个分页/批量图片HTTP顶层用例通过race；embed构建/测试与Fork通过。
父线程通知#352正常合入 `05b44bec5c2d30ad58d009fbe5247ea363c6ee70`，已经fetch确认；
已执行无冲突普通 `--no-ff --no-commit` 合并，20个主线文件逐字保留，分页源码及前端源码不变；
组合验证使用该未提交merge，门禁与浏览器复验完成后才提交。见[组合来源核对](./combination-source.json)。
此处不把初始结果当作组合已通过，最终组合SHA和验证在收尾后单独记录。

## #352组合复审

顺序执行组合规格复审、代码质量复审，两者均通过；20个主线文件及20个分页
源码/工作流文件均经独立逐字核对。分页读取不获取订阅/entitlement行锁，
未发现与#352 claim→subscription→entitlement锁序形成新增锁环；拒绝归属仍回滚
claim/计数/结算，分页未将未提交或失败结算显示为已扣费。原capture与证明谓词不变。
审查是只读检查，不代替组合门禁、最终SHA CI或生产验收；既有entitlement-ID缺口保留。

## #352组合门禁

2026-10-11组合后重新运行默认后端测试/lint、完整unit、完整协议、分页/批量HTTP race、
#352 counter HTTP/PG race、完整build、embed build/test与Fork，九项命令均退出0。
协议仍为56/56必需用例、94个顶层用例；分页/批量race为13个顶层用例，counter为
42个叶级用例（连分组共44个子项）。无测试失败。

#352没有前端修改，分页源码指纹及全部前端/依赖文件不变，故沿用初始完整前端
3611通过/8个既有opt-in跳过的结果；组合重新typecheck/build/Fork，并另行真实资源浏览器复验。
旧浏览器证据保留为[初始浏览器证据](./initial-browser-evidence.json)，不覆盖初始验证来历。
组合来源、逐门禁时间/日志校验和、精确协议集合和最终资源指纹在组合验证JSON中记录。

组合双角色浏览器于09:03:20–09:03:45 UTC通过：最终实际资源、四宽度、浅深色/中英文、
权限/legacy409、独立翻页/空中间页/503重试/刷新/请求切换，以及旧usage禁用均通过。
核对合成usage为6美元/3图，零pageerror；最新截图已替换为本次实际捕获，初始版本保留在8f427f0。
见[组合门禁/全部资源指纹](./combination-validation.json)与[组合完整协议](./combination-protocol-validation.json)。
09:01 UTC再次fetch确认远端主线仍为05b44bec；此结果不包含生产部署或用户本机验收。

## 草稿交付

[草稿PR #360](https://github.com/tqytwe/sub2api/pull/360)目标为 `play/main`。技术组合merge为
`b3df928352b8f7ee3a0acaf93d8b14752a0664f2`，两个父节点为分页初始候选8f427f0与主线05b44bec。
本节及handoff随后补齐真实PR/SHA，属于纯交接元数据，不改变已测源码/构建。
最终head与其CI以PR实况为准，由主线程核验；此处不预写CI通过或生产验收结论。


## #358 会话隔离增量

主线程确认 #358 已合入后，本分支普通合并主线
`1b701237ec3ceb869c39d28ad11d32e4065a2471`；使用其官方 session identity、publishing、blocked
及订阅接口，auth 四个文件逐字保留。没有自建登录代际协议，也没有修改后端计费或278迁移。

父页面在退出、同角色换号或同账号重新登录时同步清除列表、详情、usage、Ops浮层、过滤和精确链接；
各读取绑定身份、请求序号及取消控制器，success/catch/finally均拒绝旧结果。详情关闭同步让四个区域
失效，重开从首页开始；区域乱序和失败重试互不影响。正常令牌续期保持原页面、游标和有效在途读取。
同身份凭证正在发布时暂缓新读取，ready后恢复原意图；关闭或换号取消等待。五秒仍未ready则显示
当前区域既有可重试错误，不假定发布成功，重试保留所请求的cursor。

真实页面/Pinia/Axios/Node HTTP先复现旧页面11失败/2通过，再补ready挂载/显式读取2个RED，
以及遗留pending超时1个RED；最终48项定向通过（26 HTTP会话、19页面、3 API）。顺序规格评审
发现并关闭后两个ready边界，才进入代码质量复审。RED/GREEN原始摘要与来历见
[会话增量证据](./session-isolation-followup.json)。

构建资源浏览器脚本 [session-capture.cjs](./session-capture.cjs) 通过实际Pinia公开动作和HTTP
验证双角色的正常续期、同账号重新登录、换号失败、退出。认证端点和B账号503为loopback合成夹具，
台账读取仍经实际HTTP/PG；不得解读为生产认证、生产计费或用户本机验收。最终门禁及截图结果另见
增量验证记录，不沿用旧前端全量结果作为本轮完成依据。


本轮六项本地门禁全部退出0：make test（前端3680通过/8既有opt-in跳过）、make build、embed build/test、
Fork、design。标准双角色实际资源/PG浏览器和新增会话浏览器分别完整通过，零pageerror；
本轮协议/unit/race沿用前次组合结果的依据是后端/迁移/工作流/scripts逐字未变，不能表述成重跑。
[本轮来源与全部构建资源](./session-source-fingerprint.json)及[门禁/浏览器/审查](./session-isolation-followup.json)
绑定此次工作区；顺序规格与代码质量增量评审均PASS。首轮会话脚本logout后寻找台账按钮超时，
原因是实际router已转登录页；校正断言后双角色重跑通过，无产品修改。

09:03的旧组合浏览器JSON保存在[组合浏览器证据](./combination-browser-evidence.json)，旧截图仍可在
e818db78历史中复核；当前browser-evidence及截图是本轮真实构建重跑。生产和用户本机验收未执行。

浏览器证据复审另补退出前计数：先等既有读取稳定，再记录基线并退出，避免漏掉退出过程中误发请求。
该脚本改进已独立复审PASS，双角色实际构建/PG场景再次完整通过；没有产品源码变化。
