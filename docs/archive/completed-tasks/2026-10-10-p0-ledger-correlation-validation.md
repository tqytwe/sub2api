# P0 精确关联验证记录

状态：e209fda 的初次本地技术门禁于2026-10-10 21:17 UTC通过；草稿复审四项追加修复与最新主线组合的串行完整门禁于2026-10-10 22:12 UTC通过；草稿 PR/准确 head/CI 由提交后的交接消息记录。本记录仅覆盖隔离测试环境；未访问生产秘密，未登录 Zeabur，未生成内容、估算/补扣或变更运营。

初始基线：726289bdb757e572eac10ab5f12c689c4dc087b5。本轮复审按主线程要求普通 merge 纳入最新 play/main 17b96a20c7e9d2e394b37c6c1b108340853c0169（#353），没有 rebase/强推，签到实现与该主线一致。迁移 275 与现有计费实现保留。环境为 Go 1.27.2、PostgreSQL 18.1 一次性容器、pnpm 9.15.9 冻结锁文件、Chromium；production build + 测试 HTTP handler fixture，不是生产服务或生产验收。

## TDD 与针对性证据

先 RED 后 GREEN 的边界包括：精确 attempt 证据、旧响应/异步 Ops 引用、WS response ID、Realtime 迟到终态、同路由反向导航、drain/策略提前拒绝、binary opaque、Ops generic ID 和冷启动中英语言包。拒绝客户端 JSON 构造可信 Receipt。

真实 HTTP/PG 测试已覆盖迁移 276 重放、锁超时、4 个有效索引与旧写者空证据；HTTP200 failure、network error、空图片结果、取消/断流/重试；8 并发相同 Ops 链接幂等、跨 user/key 拒绝、原 Ops 删除后的保留引用；401/403/404 及用户字段屏蔽。数据库故障注入锁住旧 attempt、新 evidence 及 Ops links，OpenAI/Anthropic 真实 usage 原结算仍成功且精确重放不重复扣款。

补充 `requestledger.Do` 为单次调用分配私有槽，在最终 Client.Timeout 包装抹去错误链时保留原 attempt；每次实际 RoundTrip 前清空，不从 CurrentAttempt 恢复。真实 HTTP/PG + race 验证共享 client 并发隔离、失败后成功清槽、errors.Is/net.Error 与生产 ResponseHeaderTimeout。底层通用取消的既有失败分类不强改为 timeout，最终 caller 的 timeout 语义仍保持。

## 初次提交 e209fda 门禁

| 门禁 | 最终结果 |
| --- | --- |
| make test | 通过；后端普通全包测试、golangci-lint 零问题、前端 lint/typecheck、492 文件/3511 项通过，既有 1 文件/2 项跳过 |
| make test-backend-unit | 通过；包含 tag-gated 全套 unit，service 202.823s |
| make build | 通过；后端完整构建与实际前端资源，Vite 1m11s |
| Fork integrity | 全部通过 |
| 真实 HTTP/PG/WS 与 race | 上述矩阵通过；service 191.755s，repo 31.552s，HTTP 30.955s，race 10.584s |
| 实际前端与嵌入资源 | 两角色/四宽度/中英/浅深/键盘/精确反链/错误重试通过，无页面异常；embed web 测试通过 |
| 文档/交接/设计 | 全部通过；最终链接、8项交接测试、设计manifest可解码PNG |

原始日志的 SHA256 与摘要见 [validation-results.json](./p0-ledger-correlation/validation-results.json)，实际构建资源摘要见 [build-assets.json](./p0-ledger-correlation/build-assets.json)。这些是隔离技术证据，不能代替生产验收。

曾有并行编译期间 lint 被系统终止；调整运行资源（GOGC=25/GOMEMLIMIT=2500MiB）后完整门禁重跑通过，中断未算通过。另一次全量运行中，既有55ms keepalive测试仅收到2次tick（要求至少3）；未修改测试或放宽断言，独立连续20次重跑通过（1.234s），随后完整门禁重跑通过。

两处既有 unit fixture 的 gofmt 对齐（forum_sso_notify_test.go/mobile_play_billing_test.go）仅为当前 Go 工具链格式门禁，无业务行为变化。

## 独立审查

规格评审通过；独立代码质量评审在修复 drain/提前策略拒绝、binary ID 归属、冷路由文案与 Ops phase 假设后通过。补充 Client.Timeout 错误链丢失修复再次复审通过，无剩余阻塞。评审均只读，与实现者分离。

## 可复跑验证入口

- `go test -tags=integration ./internal/requestledger`：迁移、生命周期、进程终止/未知消费、证据查询；新增 Do 用例另以 race 执行。
- `go test -race -tags=integration ./internal/requestledger -run '^TestLedgerPostgres(DoTimeoutAndIsolation|ExactAttemptEvidence|ImmutableReceiptsAndLinks)$' -count=1`：真实 HTTP/PG、并发/幂等/隔离；通过 10.584s。
- `go test -tags=integration ./internal/service -run '^TestRequestLedger' -count=1 -timeout=12m`：24 项顶层回归；最终通过 191.755s。
- `go test -tags=integration ./internal/repository ./internal/requestledger/acceptance -run '^TestRequestLedger' -count=1 -timeout=12m`：原结算与诊断故障隔离、压缩流、Ops批量实际ID、真实HTTP角色边界。
- `go test ./internal/service/openai_ws_v2 -count=1`：完整 relay 包、迟到帧/drain/policy/binary 回归；通过 6.059s。
- `make test`、`make test-backend-unit`、`make build`、`./scripts/check-fork-integrity.sh`：完整门禁，结果见上一节。
- `python3 scripts/check_project_handoff.py`、`python3 -m unittest discover -s scripts -p test_project_handoff.py`、`node scripts/check-doc-links.mjs`：结构/8项测试/260份文档本地链接通过，不能验证运行态或远程引用。

## 审查问题闭环

| 问题 | 最终处理 |
| --- | --- |
| 按当前 account/attempt 猜测失败归属 | 移除 fallback，只有原 response/error Receipt 或明确 request-only 引用 |
| 旧响应恢复与内层 SmartRetry | 包装响应保留私有 Request 上下文，实际每次发送单独引用 |
| WS event.id、迟到 terminal、drain/policy return、binary | 明确协议 ResponseIdentifier；旧轮次过滤后采集 text；active-only Receipt |
| Client.Timeout 替换 error chain | 单次 Do 私有槽，实际 transport fail 写入；下一次发送清槽 |
| Ops phase 假设、冷路由语言、同组件精确反链 | generic ID endpoint、admin-ops scope、watch private_id |
| 诊断失败影响原结算 | 独立有界写入；锁故障注入验证已有真实 usage 结算及幂等 |

## 剩余边界

生产合并、部署 SHA、迁移执行、健康检查与用户本机游客/普通用户/管理员验收由主线程协调，本任务未执行。所有协议的外部真实供应商自然流量未运行；证据是隔离 fixture 与测试。历史 gpt22 损失仍未归因，此补丁只建立未来请求精确证据。


## 草稿 PR 复审追加验证

父线程只读复核的四项问题均先得到RED：真实WS/PG确认B发送后迟到A terminal导致B=RequestID resp_A；新证据表改名后detail503；关闭A再开B时A迟到覆盖B精确反链；证据行锁使健康HTTP header和stream读取各停顿约3秒，超过350ms预算。

增量修复及边界详见规格追加契约。本轮定向GREEN已包括：原始WS四组合、详情缺表降级、350ms响应预算、Ops代际、UI明确unavailable；进一步验证三轮WS八组合、辅助event.id/top-level response_id、匿名/冲突退役、严格dedicated下一轮不发送、匿名/冲突prewarm主create不发送、重用完成ID拒绝；并运行单连接/双连接池、行锁/表锁下原真实usage结算与500ms详情读取、300生产者队列上限及race。

最终日志、准确后续SHA和完整门禁以本轮完成后记录为准；未访问生产或部署。

本轮完整门禁曾因本任务构建缓存耗尽磁盘失败，清理后重跑；一次清理与尚未完成的链接竞争造成 cache file missing，也保留为失败，不计入通过。无权限绕过或生产操作。

原 handler fixture 在匿名终态换连接后按连接重置轮次，造成第二轮重复第一轮测试响应、并误报正常退役关闭。测试桩现使用跨连接 atomic 轮次、显式允许正常退役、收齐每条连接的完成结果；保留 9/4 和 18/8 token、唯一计费 ID、无重放断言，新增匿名池模式 2 条物理连接、正常/透传 1 条断言。独立复审确认无弱化。

本轮独立规格评审与代码质量评审均通过。追加审查确认池上限不会提前驱逐活动 lease，旧 generic event.id 不参与新归属，严格续链 shared/dedicated 保留前轮 usage 后拒绝下一轮，prewarm 歧义不发送主请求；最终完整门禁结果见下节。

另一个旧 service fixture 在匿名终态后重复拨出已关闭对象，完整包确认单项失败。改为两条真实独立测试连接，保留第二轮无 previous_response_id、响应 ID 与正常关闭断言，并断言每条连接各发送一次；独立代码复审通过。此项仅适配连接退役契约，未再改业务实现。已通过的相同业务源码 HTTP/PG/race/build/browser 结果复用，完整门禁串行重跑，活动构建期间不再清理 Go 缓存。

## 本轮最终门禁（组合 #353 / 17b96a2）

| 门禁 | 实际结果 |
| --- | --- |
| 匿名终态测试桩适配 | exit 0；独立复审未弱化断言 |
| make test | exit 0；后端全包 service 135.171s；golangci-lint 零问题；前端 lint/typecheck，Test Files  493 passed | 1 skipped (494)；Tests  3528 passed | 4 skipped (3532) |
| make test-backend-unit | exit 0；全包 tag-gated unit，service 196.747s |
| 完整 make build | exit 0；组合主线后的实际前端构建 42.95s；后续只有测试桩/文档变化 |
| Fork integrity | exit 0；后续两处 WS fixture 不在其运行选择范围，其验证目标源码未变化 |
| 全量 requestledger PG + race | exit 0；104.450s；含 300 生产者有界队列 |
| 全 service Ledger PG | exit 0；26 顶层测试，136.706s；真实池化 WS、迟到帧与退役矩阵 |
| repository + HTTP/PG | exit 0；22.664s / 32.625s；小连接池、行/表锁、缺表、权限与原 usage 结算 |
| handler observed-usage race | exit 0；3.477s；跨连接轮次、每轮 token、唯一计费 ID、无重放 |
| CheckIn 组合 HTTP/PG | exit 0；12.602s，含真实前端合同 |
| 新构建浏览器 / embed | exit 0；双角色四宽度/中英/主题/键盘/反链/读故障；embed 0.197s |

常规前端运行跳过的 4 项均为 CheckInView.http.spec.ts 的真实后端条件测试；已由 CHECKIN_FRONTEND_CONTRACT=1 的组合 HTTP/PG 合同单独运行并通过。

`validation-results.json` 的 `followup_checks` 与 `followup_failures` 分开记录日志 SHA256、真实退出码及摘要；初次 e209 证据仍保留为历史，未冒充本轮结果。最后三项重型门禁按同一队列串行执行，未再清理活动缓存。PG/race/build/browser 的业务源码与最终补丁相同，后续测试桩调整单独验证，不重复运行无变化的检查目标。

独立规格及最终测试桩代码复审通过；PR 准确提交 SHA 与 GitHub CI 状态由提交后读取，文档不自指一个尚未生成的 SHA。仅草稿 PR，不合并或部署，不把本地验证代替生产验收。
