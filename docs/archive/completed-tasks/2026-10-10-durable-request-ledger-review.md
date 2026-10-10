# 持久请求台账：规格与质量复核记录

范围：独立 worktree `codex/durable-request-ledger-20261010`，初始审查基线 `3843ff3e931349595b8793b52504b02a177f12c9`，后按根线程授权快进到 `7291ae9a2be4db7d97b8b641d053f7822276dc23`。
本记录是该实施线程的两轮检查，不冒充外部 reviewer 的独立批准。根线程负责组合审查、发布和生产验收。
规格见 [台账规格](./2026-10-10-durable-request-ledger-spec.md)。

## 第一轮：规格检查

- 私有服务器 UUID 与客户端 ID、上游 ID、收费幂等键分离；入口在 auth/body/CORS/路由 handler 前同步 PG 提交。
- 已验证 auth 才写数字 user/key ID；匿名拒绝也保存。前端普通用户范围在 SQL 里强制，管理员另有角色检查。
- 每个应用可观察的上游发送有预提交 attempt；实际 HTTP 重试位于兼容 transport 内层，调度/母账号分开。
- WS turn、连接/control、输入流和自动观察 turn 的阶段明确区分；异步入队意图与后台执行分别保留父子关系。
- 执行、用量、结算独立；不把缺失 usage 改成 0，不因成功重试覆盖之前 attempt 的未知消费。
- 已有 settlement/dedup、usage、钱包、订阅及套餐引用保留；台账不产生新的收费策略或金额。
- crash recovery 仅把过期实例的记录标记中断/未知；不重发、不补扣、不补历史，不删除。
- 受管路由拒绝覆盖不等于完整供应商协议成功覆盖。Realtime/WebRTC/plugin 内部的观察边界在规格逐项标注。

发现并修复：usage worker detached context 会失去私有请求归因；SSE 失败终态会被后来的成功标志覆盖；
第二笔 billing intent 会错误继承上一笔 settled；成功 retry 会隐藏第一次未知消费；图片幂等重放未关联实际旧任务；
管理员/用户请求路由漏加 lazy locale scopes；英文筛选丢失语言参数；流式响应已写入 200 后 panic 会被误记为 HTTP 500；Anthropic/Gemini 会把台账故障转为上游 failover；Realtime 跟踪容量拒绝在独立 turn 持久化之前发生（两类 PG 用例 RED→GREEN，05:54:52 UTC exit 0）。
合入 PR346 后追加发现：原始 terminal 不能自行结束 passthrough 当前 attempt；迟到的 WS usage 不能标记重试后的最新 attempt。两项真实 PG RED 于 06:05:15 UTC 证实，按 relay 权威 turn 回调和精确 attempt 编号修正，未复制响应 ID 归因或收费算法。GREEN 于 06:08:31 UTC：service 21.550s、ledger 4.553s；随后统一执行合并后的最终门禁。

## 第二轮：代码质量检查

- DB mutation 参数化；安全 error 枚举、方法白名单及路由模板长度约束，不存正文、原始 URL、query、Authorization、Cookie、IP 或 UA。
- 每次写入与查询限时 3 秒；响应开始前存储失败只拒绝该请求；不改全局生产配置。已开始输出后无法改写 HTTP 状态，已有证据继续保留。
- 实例 lease 由 PG 时间决定。新一代只接受新请求，过期 WS/worker 父实例不能被新 turn 复活；重复终态 first-write-wins。
- attempt 序号在行锁事务内分配；output 标记每 attempt 一次；SSE metadata 缓冲最多 64 KiB/行且不持久化正文。
- billing 验证和原 monetary/dedup effects 在同一事务中；验证失败回滚收费。异步任务 wallet reconciliation 只读现有 capture/release 证明，hold 不等于 settled。
- 后台 recovery 和 task reconciliation 每轮最多 1000 条，按检查时间轮转，避免一直卡在旧待核对项；无自动删除。
- 只读 API 分页有上限、排序稳定；详情及 usage 原记录二次验证 owner；普通用户隐藏调度/母账号 ID。
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

## 最终门禁

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

实现代码在最终 Go/PG/全量测试与构建后未再修改；之后仅补审查记录和刷新截图。完整 CI 与准确 commit 见本 PR，生产仍由根线程处理。

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
