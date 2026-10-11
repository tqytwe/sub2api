# P1 原生自动续期与福利登录态同步

状态：本批隔离技术门禁及双审查已通过；草稿 PR/准确 head CI 待交付，不代表生产验收。

初始基线 `play/main`：`1bfc7e2edb788ca11a23ff936f99db904c4b4199`。发布前按主线程要求以普通 merge 纳入最新 `23d06337cbc315cb92d3fc18e94902fe21e7342d`（#356），不 force/rebase。分支：`codex/p1-auth-refresh-20261011`；worktree：`/workspace/p1-auth-refresh`。

## 问题与边界

真实 Axios 续期先持久化旋转令牌，store 仍按旧令牌判断响应是否属于当前会话，因此提前返回而未更新 Pinia。答题/盲盒会把正常续期误认为会话变化，资料余额也可能无法更新。本批删除续期后的 `checkAuth()` 补救，直接验证定时器及 401 路径。

只修改 `frontend/src/api/client.ts`、`frontend/src/api/tokenRefresh.ts`、`frontend/src/stores/auth.ts` 三个生产文件。发布不携带凭据的同文档通知同步 Pinia；使用内存会话代次和用户 ID 防止旧 401 在新会话下重试、旧资料响应覆盖新用户、旧失败退出新会话。同会话正常资料变化不构成切号。定时续期使用原始 Axios，硬拒绝执行现有失效策略；网络、429、5xx 保留会话。

本批 Auth 修复未改变令牌期限、权限、后端路由、租户边界、计费或福利规则；未新增凭据存储或迁移，Auth 差异未涉及 SQL275/276、P0 台账及 upstream 关联；普通 merge 原样保留 #356 的代码、CI、迁移277和文档成果。隔离夹具发现的并发问题不代表已证实生产跨账号泄露。无需页面原型：本批不修改页面、组件、模板、样式或文案；福利文件仅增加/修正测试。

## TDD 与真实传输证据

原始 RED 及 SHA256 见 [verification.json](./verification.json)。

| 阶段 | 实际结果 |
| --- | --- |
| 原生答题自动续期 | 2 RED / 5 skipped；令牌已持久化而 Pinia 仍旧值 |
| 初始真实 HTTP 会话合同 | 11 RED / 5 passed；无 Axios 错误结构 mock |
| 已入队重试后切号 | 1 RED；会话错误被错误归一为 status 0 |
| 独立规格审查 | 定时硬拒绝 401/403 未退出；同账号资料变化使 503 误转 401 并退出 |
| 正式审查回归 RED | 上述 3 行为 RED；另有观察屏障 1 秒与既有 peer 等待 1 秒相撞，观察期限改为 2.5 秒，生产等待不变 |
| 修复后定向回归 | 119 passed / 6 files：auth HTTP 27、auth unit 24、client 30、coordinator 7、答题 session HTTP 7、盲盒 HTTP 24 |

真实 HTTP 使用回环 Node server、真实 Axios adapter、生产 Pinia/coordinator/client，账户及令牌均为确定性合成夹具。Web Locks 能力由测试适配，刷新请求仍走真实 HTTP；既有 coordinator 单测保留跨标签页 peer 分支覆盖。覆盖三个并发 401 一次续期并分别重试，定时续期并发、下一次定时器、新会话可独立续期、logout/切号/同账号重登的晚成功和晚拒绝、旧 profile 200/401、失败退出及临时失败保留。

## 实际构建浏览器与 PG

[browser.py](./browser.py) 只加载 production build，拦截非回环网络。四组：答题和盲盒各自定时续期/401重试，360/1280、浅色/深色、中英文。启动仅植入合成登录夹具，之后不调用 checkAuth、不刷新页面、不手动更新 token/余额。检查 Pinia/storage 一致、结算余额、结果页、单次 fixture commit、盲盒重试 key 一致与提交后清除、无 JS 错误及横向溢出。报告和截图在 [browser-evidence/report.json](./browser-evidence/report.json)。浏览器 fixture commit 不是 PG 实际记账。

真实 PG 独立执行既有 `scripts/test-welfare-contract.sh` 与 `scripts/test-checkin-contract.sh`：正式 handler/service/repository、隔离 PostgreSQL 全迁移、HTTP/Axios/Pinia/Vue，校验幂等、防重、权限拒绝、原子持久化及确定性余额。身份为测试 allowlist；不使用生产账号，不访问生产，不重发奖励。默认前端全测跳过的 PG 文件必须由这些脚本另外执行，不能算默认已通过。

## 交付门禁

额度中断恢复后核实：PG、Fork、设计/文档/交接、完整构建及最终浏览器均已通过，源码审查哈希一致，未重做这些有效检查。首轮 `make test` 在既有后端 `TestServerTimingConnectorRecordsDriverCallsWithoutRowLifetime` 计时断言失败（app30.2ms、db39.1ms）后停止，前端全测及tag-unit未执行。原失败保留；低并发独立复跑3/3通过，未改断言。后续完整重跑后端/lint与前端lint/typecheck通过；前端3625通过/1失败/8默认跳过，发现旧签到切号夹具只赋token即期待另一用户资料替换账号，被新守卫正确拒绝。将夹具改为真实setToken会话入口，保留旧资料不能覆盖新账号断言，63项相关HTTP通过。该测试修正不改生产源码。纳入23d06337后在组合版本重新执行完整门禁。

组合版本 `make test` 已通过：后端默认测试/lint、前端lint/typecheck、前端3630通过/8跳过（500通过/3跳过文件）。默认跳过为签到HTTP5项、答题PG2项和盲盒PG1项，均已在独立PG合同脚本执行，不冒充默认全量通过。`make build`、Fork、设计/文档/交接检查、浏览器4/4、隔离PG签到及福利合同均通过。`make test-backend-unit` 于2026-10-11 07:33:59 UTC通过。最终门禁、原始失败与源码hash记录在 [verification.json](./verification.json)，顺序独立审查在 [reviews.md](./reviews.md)。目标 play/main 的草稿PR及准确head CI在PR正文记录；本次提交前不提前计成功。

本批不合并、不部署。前批 #357 普通合并 `1bfc7e2e` 并部署成功由主线程报告，本批未访问生产复核。用户本机三身份验收待后续部署。组队、擂台、推广、券包及运营页未按本批标准验证，不宣称通过。
