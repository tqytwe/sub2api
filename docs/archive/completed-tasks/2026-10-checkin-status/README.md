# P1 签到读取状态与中枢刷新

基线：`17b96a20c7e9d2e394b37c6c1b108340853c0169`（#353 普通合并）。独立分支 `codex/p1-checkin-status-20261010`。本批不合并、不部署、不访问生产；最终 head 与 hosted CI 以本批草稿 PR 为准。

## 已复现问题与改动边界

- CheckInView 把 GET 失败置为 null，模板随后显示“签到功能暂未开启”。实际 Axios 将 HTTP 状态放顶层；现在读取其 status，分别显示加载、关闭、403 权限、网络/超时及其他读取失败，并提供重试。
- `next_milestone_days` 是绝对目标天数（play_streak.go 的 resolveNextMilestone），不是剩余天数。真实 PG 六天连续记录返回 target=7；页面应显示再签 1 天。只做展示减法，不改目标、金额或任何福利规则。
- PlayHub 首次加载失败后 hub=null，使原有 `loading && hub` 守卫允许重复重试。两个页面现在单请求加载、禁用刷新并移除过期福利操作，组件卸载后忽略旧读取结果。
- 独立规格审查发现另一标签页发奖后的旧余额：真实 PG RED 中现金/里程碑应显示 $1.50/$2.50，实际仍是 $1.00。现在状态读取成功后通过 authAPI 只读取得本页余额快照，再一起呈现页面；读取失败/加载期间余额用 —，不会将缓存余额当作当前余额。真实 Axios→AuthHandler.GetCurrentUser→UserRepo/PG 验证；成功 POST 后保留已有共享账户刷新，非关键刷新失败不撤销奖励成功提示。本页只读资料失败显示读取错误，不重复 POST。另用真实 Pinia＋HTTP 验证晚返回资料、切换账户及读取失败。
- 第二轮独立审查发现直接用共享 authStore.refreshUser 读取余额会在旧页面卸载后写全局缓存；旧构建的真实路由浏览器 RED 为新 $2.50 回退旧 $1.00。改成本页只读资料和余额快照，卸载/会话变化均不应用；未扩改公共认证并发策略。
- 质量审查发现 token 自动刷新可在 POST pending 期间发出旧 GET；普通/补签两条真实 Pinia/HTTP RED 均仅有 2 次读取而缺少提交后第3次。现在奖励操作需要重读时标记后续读取，丢弃旧在途结果再取得提交状态；手动重复点击仍单请求，不增加多次刷新。
- 签到/补签与手动刷新同一 tick 的竞态：仅 disabled 属性不足以阻止旧 GET；手动刷新入口同时检查提交状态。成功或重复签到仍按既有流程读取已持久化结果。

沿用 AppLayout、growth-world 面板和按钮、现有 Toast、中文/英文文案体系；只改两个用户页及福利文案。后端改动全部是测试。公共 Axios、路由、SQL275、P0 请求台账、attempt/Ops/upstream、价格、计费及福利规则均未改。

## 证据与限制

- `WelfareStatus.http.spec.ts` 用实际 Node HTTP 服务、playAPI、Axios transport/interceptors 注入 503/403/socket close/真实超时；没有直接 mock 错误对象。覆盖重试恢复、关闭重开、同 tick 重试、加载时移除旧操作、离开重开后旧响应及剩余天数。首次 RED 确認 503/剩余天数等失败；早期夹具在用例之间关闭 keep-alive socket 导致额外网络错误，已用 Connection: close 隔离，未改生产客户端。
- `CheckInView.errors.spec.ts` 继续覆盖全部已知 POST 错误分支，新增普通/补签 pending 时立即点击刷新守卫。该新增断言先失败再修复。
- `./scripts/test-checkin-contract.sh` 用全量迁移的隔离 PG、实际 routes/service/repository/ledger、真实 Axios/Vue。测试认证是合成用户 allowlist；GET 的 503/403 为显式测试故障，其他读取及全部奖励走实际服务。独立 disabled service 验证实际关闭→重开。只读故障用户最终 0 签到/0 快照/0 能量/0 现金、余额仍为 fixture 的 1；普通/里程碑/补签、重复和并发由 PG 对账确认。中枢与签到对比已签到、连续天数、pending_actions 和实际余额，并将真实 authStore 保留为旧余额验证本页只读刷新；Explorer streak 字段为空/0 是当前规则，未改为现金 streak。
- `browser.py` 只加载 production build，提供隔离回环 HTTP 夹具并阻断外部浏览器请求。PNG 是真实 Chromium 截图；prototype 是实现前 DOM 预览，不能当回归证据。浏览器夹具不等于真实 PG，PG 证据来自上一层独立合同。
- `baseline-assets.json` 对应 #353 构建，其 tree 与本批 merge 基线均为 `1d9285de85d071d08c51c7b2e4ff778722ecea16`。更新构建哈希、逐例浏览器报告及门禁记录同目录。
- 余额一致性覆盖两个福利页面的主余额面板；全局顶栏仍采用既有 auth 缓存策略，外部标签页发奖后可能晚于本页只读快照更新。公共认证缓存的并发/刷新策略不属于本批，未改动或宣称已解决。
- 不宣称其他福利页面、生产、本机游客/用户/管理员验收通过。失败 GET 不触发奖励；只使用测试账号和隔离数据。

## 已完成专项验证

43/43 目标用例、5/5 PG live Vue、最终 make build、11/11 构建浏览器场景、设计治理、263份文档链接、交接结构及8项交接校验测试均通过。顺序独立审查见 reviews.md；合入 #354 前的完整本地门禁结果见 verification.json，准确 head/hosted CI 见草稿 PR。

合入 #354 前的完整门禁：make test（前端 495 文件/3541 项通过、5 项默认 live fixture 跳过）、unit 62 包、integration 58 包/16660 测试及子测试通过/23 项跳过、Fork integrity 全部 exit 0。integration 中 5 项 live Vue 无跳过；完整跳过原因保留在 verification.json。

环境曾因 /tmp 配额耗尽中断一轮全量 integration 的日志；该轮不计通过。只将本任务Go缓存迁入 /workspace 后恢复空间，最终重新调用全量 suite；39 个未变包复用 Go 缓存，修改过的 PG/HTTP/Vue 合同实际重跑。没有清理源代码、其他任务或生产数据。

## 与 #354 的主线组合

P1 首提交 `7cdab5b6b` 完成上述合并前门禁；随后按父线程指示普通合入 `9b367c378b04db987a36175af0b8a3e02e22da24`，未 rebase/force。三处共享路径为 FORK_CUSTOMIZATIONS、docs/README 和 handoff，均自动合并无冲突。其余 134 个主线新增/修改路径与主线逐字节一致；后端相对新主线仅签到测试夹具有差异，SQL275/276 与主线一致。

完整本地门禁属于合并前实现提交；组合后按影响范围复核实际 PG（包括主线新增迁移）、HTTP/Vue、前端目标测试及冷路由语言范围，并重新构建前端、运行全部 11 组浏览器场景。组合后 54/54 目标用例、5/5 PG live Vue、前端构建与 11/11 构建浏览器、7 项治理单元和设计治理、266 份文档链接、handoff 结构及 8 项校验全部通过。组合结果见 combination-verification.json，文件归属证明见 combination-scope.json。未重复执行未变福利源码的昂贵全量本地门禁；最终准确 merge head 的完整 hosted CI 单独记在草稿 PR。

`premerge-assets.json` / `premerge-browser.json` 保留原基线构建验证归属；`pre-session-fix-assets.json` / `pre-session-fix-browser.json` 保留 56e58ed8 组合构建；`updated-assets.json` / `updated-browser.json` 和更新 PNG 对应会话隔离补丁后的最终重构建。最终组合按原顺序再做独立规格、质量复审，记录见 reviews.md。人工截图复核发现动画中间帧后，仅补强截图夹具等待有限动画自然结束并断言 banner opacity≥0.99；质量复审及最终 11/11 重拍通过，未修改样式或业务源码。

## 父线程复审补充：同组件中枢会话隔离

父线程复审旧 head `56e58ed83707ebcc159f0e9f545f6f4ab454cbe9` 时发现同一 Hub 组件仍可应用 A→B 的迟到私有响应；该 head 随后 7/7 CI 通过，但之前的会话测试只覆盖签到，不能证明中枢通过。新真实 Pinia/Axios/HTTP 用例先取得有效 RED（7失败、1自动401刷新对照通过），旧生产构建浏览器也复现 B 的顶栏$2.00而中枢回退A的$1.00。

最小修复仅在 PlayHub 增加 user/token 监听和请求代次：清空旧 Hub/邀请活动，当前账号立即重读；success/catch/finally 都拒绝旧代次，登出不发私有读取。另追加 A→B→A 同 id/token 回归，保留同会话重复点击守卫。没有修改公共 auth/Axios/tokenRefresh，真实续期测试明确区分 token 的实际自动刷新、持久化及 store 随后采用；实际401刷新重试有独立对照，保证成功读取仍可完成。

两轮顺序独立源码审查均通过。最终补丁63/63目标、5/5 live PG/Vue、496文件/3558项完整前端、lint/typecheck、前端构建与14/14实际资源浏览器场景均通过（默认5项 live PG跳过已另实际运行）；适用门禁和结果见 `session-isolation-verification.json`；旧完整后端门禁保留原 SHA 归属，没有因前端补丁重跑未变后端全套。新 head 的全部 hosted checks 单独记录在草稿 PR，旧 CI 不能替代新补丁验证。仍保持草稿、不合并部署，不将本地技术验证视为生产或用户本机验收。

## 下一批：先答题，再盲盒

本批只盘点以下证据，未修改这些页面，也未将已有单层 mock 作为跨层验收。

| 页面 | 当前代码/已有证据 | 下一步窄范围验收 |
| --- | --- | --- |
| QuizQuestView | loadQuiz catch 将 quiz=null，模板 `!quiz?.enabled` 显示关闭；这是源码嫌疑，尚未真实 HTTP 复现。现有 QuizQuestView.spec.ts 覆盖奖券刷新、切账号旧响应、账户刷新失败等，但 mock playAPI。未找到独立 quiz repository PG integration 文件。 | 先实际 Axios GET503 RED；隔离 PG 建题目/用户/资格，正式 HTTP 提交→结果刷新→中枢。测试空题库、非法答案、当天并发与重复、资格拒绝、timeout后查询已提交结果，核对 attempts/奖励/能量/余额唯一性；只修复有证据的最小缺陷。 |
| BlindboxView | 已有 statusLoadFailed/retry 和会话隔离；BlindboxView.spec.ts 覆盖身份/幂等 key 持久化/旧响应/加载失败。play_blindbox_open_integration_test.go 有串行余额日限、同 key 防重、ledger失败回滚，但尚未连到真实 Axios/Vue。 | 复用隔离 PG，接正式 HTTP 与 Vue，验证游客公开池边界、已发布/耗尽/未发布、同 key timeout重放、不同key并发、日限/余额不足、券/码恢复与过期；对账扣费和发奖同事务。与中枢核对 opens_today/can_open，记录实际构建加载/空/错/重试。 |

下一批从最新主线另建分支，先题目 GET 错误复现；盲盒单独追加可审查的合同测试，不并入本批重写。涉及公共文件先协调。真实 PG 中固定夹具金额仅用于等值断言，不估算生产资金、不调用生产奖励接口。
