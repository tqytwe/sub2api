# P1：已准入的文本请求在订阅撤销后结算

原始 RED 基线为 `fdec971d2a01c25d93874b7688dffcff78f79088`；交付前已普通快进合入 `fb2c9747b58ed5d980a36ea6a2316ead2aa03b7e`（包含 #360 分页与迁移 278），随后普通快进合入纯认证前端更新 `6ae024e6b732b64b2ce6f7ba56c5be4e20b88bdb`（#362）；分支 `codex/p1-inflight-revoke-20261011`。本批只验证独立 PostgreSQL 与合成 HTTP 上游，没有访问生产或发送付费生成。此记录不代表已部署或用户本地浏览器验收。

## 契约

本批“原生”按 OpenAI 平台的入站 Responses HTTP 文本接口判定（流式及非流式、token/按次渠道价）。既有内部 HTTP 协议兼容转换保持原逻辑，每次实际发送同样复核准入。请求在每次真实发送前，锁定原订阅及套餐、复核状态/到期/额度，并绑定 server request、真实 attempt、用户、key、group、account、原 SubscriptionID/PackageEntitlementID、原请求 hash 与版本化价格快照。原有 request/attempt/evidence/billing links 和 legacy fingerprint 保持原职责；不建立第二套准入 attempt 或账本。

撤销先提交则不能发送；准入先提交且随后取得真实 usage，可以按原价格向原对象结算。撤销阻止新的 failover、换号和 compact 兼容重试。曾经有未知用量的尝试仍保持 unknown，后续一次有证据的结算不能推断先前尝试免费。一个私有 server request 只允许一份真实 usage 与命令；不同请求即使使用相同客户端 request header 仍分别结算。

自然到期、并发耗尽、错误归属、缺少/损坏准入证明不获得撤销豁免。首个撤销事件保存先前状态和有效额度，后续 Restore/reset/续套餐不会隐藏之前的耗尽或改绑新套餐。精确的已完成 replay 可跨撤销/到期复核，不能再次增加计数器。

计数口径保持：普通订阅 daily/weekly/monthly 各加 SubscriptionCost；套餐 request_used 加 1、amount_used_usd 加 BilledCost、token_used 加四类 token（input/output/cache creation/cache read）。旧精度规则及金额 fingerprint 不变，完整 payload 另有版本化 SHA-256 绑定。

## 持久化与恢复

迁移 280 新建 receipt、不可变业务生命周期事件和 outbox 三张表。没有历史回填、推测用量或历史重计费。

价格快照显式保存 ResolvedPricing 的两个私有字段、区间/时段/层级价、计费候选及采用资格、个人/组/账号倍率、附加费、Free Fast 和长上下文策略。账号统计只保存所需定价规则。原始 prompt、响应正文和凭据不进入快照。

取得真实用量后，同步保存原 usage、原完整命令和显示投影，再执行 Apply。首期路径同步完成这一步和 Apply，故不依赖内存 usage worker 队列。进程中断后，生产构造器启动的有界恢复 worker 只领取原命令并重入原 Apply；不重发、不读取新价格。六计数器、dedup、billing link、usage settled 投影和 outbox completion 在同一事务提交；失租和任一 DB 错误均回滚计费事务。

数据库无法保存观察时不伪标 known，也不扣费。未识别的响应计费模型、未知 tier 或输出中的媒体/工具用量保留已观测数字为 pending，不能自动恢复成免费账单。输入文件、音频、图片、工具及非原生文本路径不获得本批撤销例外。原生文本 reasoning/compaction 历史项允许进入既有兼容重试，但每次实际发送重新准入。

普通订阅合法自动 reset 复用既有日历日、周/月滚动及一次性窗口规则，在同一订阅行锁内推进；套餐不继承普通窗口限额。没有新增透支、额度预约、退款或运营定价规则。

## 可复用运行入口

```bash
./scripts/test-subscription-counters.sh '^TestSubscriptionInflightRevoke$' -race
./scripts/test-subscription-counters.sh '^TestSubscriptionCounterHTTP$' -race
./scripts/test-subscription-counters.sh '^TestSubscriptionEntitlementBindingReproduction$' -race
./scripts/test-subscription-counters.sh '^TestSubscriptionBindingCompatibility$' -race
make test
make test-backend-unit
make build
./scripts/check-fork-integrity.sh
```

脚本只创建带测试标签的独立 PostgreSQL 容器，执行完整迁移；端口占用时失败，不复用、清空或删除其他数据库。测试中的时序使用显式 gate 和 PostgreSQL `pg_blocking_pids` 等待图，不能以 goroutine 启动先后来冒充锁顺序。


## 隔离验收证据

完整运行日志保存在执行环境 `/workspace/p1-inflight-evidence/`，核心 HTTP RED/GREEN 与组合回归已压缩归档在 [evidence 清单](./evidence/log-manifest.json)。清单分别记录解压后与压缩文件的 SHA-256，供核对原始字节；新 CI 步骤会上传 `subscription-inflight.log`，不能把静态审查当作运行闭环。

| 证据 | 运行基线与结果 | 范围 |
| --- | --- | --- |
| 初始契约 RED | `fdec971d`，12 场中 8 RED / 4 正向对照通过 | 撤销仍发送、撤销后已准入结算被拒、倍率未冻结 |
| compact 统计 RED | `fb2c9747` + 修复前单文件 overlay，完整迁移 406 | 实际 `gpt-5.5` 出站，客户按 `gpt-5.1`，统计价丢失导致 `account_stats_cost=NULL` |
| 最终在途回归 | `6ae024e6` + 本批源码，56/56，完整迁移 406，`-race` | HTTP/auth/真实发送/usage/Apply/六计数器/proof/用户与管理员 API、锁等待、冻结价格及持久恢复 |
| 组合旧计数器回归 | `fb2c9747` + 本批源码，42 个叶子场景，完整迁移 406，`-race` | 普通/套餐、幂等、并发、reset、回滚与展示 |
| 冻结绑定/兼容回归 | `fdec971d` + 本批源码，4 + 36 场，完整迁移 405，`-race` | 复用合入 #360 前证据；该批后续只改变 scoped 定价及测试等待，不改变这些旧调用方的绑定/Apply 路径 |
| Live 异步测试等待 | 原断言 100 次有 1 次竞态；修正等待后 100 次通过 | 仅修测试等待 Apply 后的实际释放，保留 calls=2、release=1，不改 Live 生产行为 |

最终在途日志 SHA-256：`ac396fa68b8c4b49b1550d866bfd4699ec1af03878b319896481f1027cb9d33b`。有效 compact RED 日志：`d145b99e703f7f5af8d7519357c5ecce2c7b3d6b1ec08a74097282d0dba9a7a6`。更早的无默认价格夹具失败、磁盘不足、尚未等待 lease 释放的恢复测试失败均保留为诊断，不计为业务 RED 或最终通过。

规格审查由 `/root/p1_spec_review` 只读完成，限定 PASS，无未闭合 P1/P2；未独立重跑。受审 27 个非文档变更文件的[源码清单](./reviewed-source-sha256.txt)摘要为 `e87c8b748d3e1981a0d8b8b21884d69e5a3d10a4d985e5d83dcba139d621d179`。随后由 `/root/p1_quality_review` 独立只读完成质量审查，PASS，无新增 P1/P2；未独立运行测试。两项审查依次执行，源码清单完全匹配。8 项服务器门禁全部通过，命令、时间及原始日志哈希见 [门禁记录](./evidence/gates.json)：完整测试、unit tags、ledger integration、完整构建、Fork、带标签增量 lint、文档链接及 diff 检查。ledger integration 有 461 个 PASS 节点；3 个 browser fixture/子进程 helper 按既有条件跳过。

## 交付边界与回退

生产构造器接入恢复 worker 的行为仅在隔离 fixture 中运行验证，不能据此声称生产已启用。崩溃场景使用确定性故障注入；失租场景验证 CAS 回滚，不声称实际进程 kill 或所有多 worker 调度组合已覆盖。

迁移 280 为增量表及索引，旧写入方不使用这些表。源码回退会停止新 receipt/outbox 的生成与自动恢复，已有待结算命令必须保留并交由主线程协调；不删表、不重算历史、不据无证据窗口补扣。权限放宽、计数不一致或迁移错误应停止发布并协调审查过的源码回退。

合并、部署、生产健康检查及用户本地浏览器三身份验收由主线程/用户后续协调；本批没有执行，也不以隔离 HTTP 测试替代。


## 后续纯前端基线组合

#362 的 merge `6ae024e6`（父 `fb2c9747` + `df28f49d`）仅改变认证前端、测试及归档；`backend/`、迁移、CI 与脚本相对旧基线没有差异。本批 27 个受审源码/CI/Fork 检查文件合入后逐一哈希相同，因此按主线程授权复用此前组合证据，不把旧计数器与绑定证据写成新基线重跑。

新基线补跑认证组合 6 文件、211/211 测试通过。PR #362 的准确源 head `df28f49d0c3c355a622cc753055f4b63da4e3e9e` 六项 CI 全为 SUCCESS；这是来源证据，与本批后续 CI 分列。完整 `make test` 在 `fb2c9747` + 本批源码通过（前端 502 文件/3680 测试 PASS，3 文件/8 测试既有 skip）；后续完整构建、Fork 与相关 integration 使用合并后的工作树。

纯前端基线合入后的增量规格、质量复核再次依次 PASS。最终增量 lint 清理仅删除未使用的测试 helper、简化等价字段访问；两位审查者再次依次只读确认其余 26 文件不变、断言及逻辑不变。清理后在 `6ae024e6` 基线上重跑最终 56/406/race，11:52:39 UTC 完成，见 [最终运行记录](./evidence/commit-validation.json)。
