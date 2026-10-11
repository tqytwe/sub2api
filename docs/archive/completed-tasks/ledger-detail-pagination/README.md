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

不修改计费写入、capture、usage认证条件或
`backend/internal/repository/usage_billing_repo.go`；不引入补扣、估算、TTL、
历史清理或猜测关联。历史 SQL/checksum 不变；新增278并发索引支持按request/task定位来源。
固定上游仍为 ranxi v2.10.3。

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

最终全量门禁、HTTP/PG与真实资源交互、顺序规格/代码复审、PR/SHA/CI待补。
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
接下来普通合并、保留其repo修复与测试，并验证组合。此处不把初始结果当作组合已通过。
