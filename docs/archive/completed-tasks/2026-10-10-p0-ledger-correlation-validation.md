# P0 精确关联验证记录

状态：本地技术门禁与独立评审全部通过，2026-10-10 21:17 UTC；草稿 PR/准确 head/CI 由提交后的交接消息记录。本记录仅覆盖隔离测试环境；未访问生产秘密，未登录 Zeabur，未生成内容、估算/补扣或变更运营。

基线：726289bdb757e572eac10ab5f12c689c4dc087b5，2026-10-10 再次 fetch origin/play/main 确认。迁移 275 与现有计费实现保留。环境为 Go 1.27.2、PostgreSQL 18.1 一次性容器、pnpm 9.15.9 冻结锁文件、Chromium；production build + 测试 HTTP handler fixture，不是生产服务或生产验收。

## TDD 与针对性证据

先 RED 后 GREEN 的边界包括：精确 attempt 证据、旧响应/异步 Ops 引用、WS response ID、Realtime 迟到终态、同路由反向导航、drain/策略提前拒绝、binary opaque、Ops generic ID 和冷启动中英语言包。拒绝客户端 JSON 构造可信 Receipt。

真实 HTTP/PG 测试已覆盖迁移 276 重放、锁超时、4 个有效索引与旧写者空证据；HTTP200 failure、network error、空图片结果、取消/断流/重试；8 并发相同 Ops 链接幂等、跨 user/key 拒绝、原 Ops 删除后的保留引用；401/403/404 及用户字段屏蔽。数据库故障注入锁住旧 attempt、新 evidence 及 Ops links，OpenAI/Anthropic 真实 usage 原结算仍成功且精确重放不重复扣款。

补充 `requestledger.Do` 为单次调用分配私有槽，在最终 Client.Timeout 包装抹去错误链时保留原 attempt；每次实际 RoundTrip 前清空，不从 CurrentAttempt 恢复。真实 HTTP/PG + race 验证共享 client 并发隔离、失败后成功清槽、errors.Is/net.Error 与生产 ResponseHeaderTimeout。底层通用取消的既有失败分类不强改为 timeout，最终 caller 的 timeout 语义仍保持。

## 最终门禁

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
