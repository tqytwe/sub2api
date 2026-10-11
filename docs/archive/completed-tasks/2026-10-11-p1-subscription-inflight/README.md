# P1：已准入的文本请求在订阅撤销后结算

原始 RED 基线为 `fdec971d2a01c25d93874b7688dffcff78f79088`；交付前已普通快进合入 `fb2c9747b58ed5d980a36ea6a2316ead2aa03b7e`（包含 #360 分页与迁移 278），随后普通快进合入纯认证前端更新 `6ae024e6b732b64b2ce6f7ba56c5be4e20b88bdb`（#362），最终普通 merge 合入 `8364b462feda15f3bf8f3bb2029314d2193b9dba`（#359 选路审计及迁移 279）；分支 `codex/p1-inflight-revoke-20261011`。本批只验证独立 PostgreSQL 与合成 HTTP 上游，没有访问生产或发送付费生成。此记录不代表已部署或用户本地浏览器验收。

## 契约

本批“原生”以实际出站 OpenAI Responses HTTP 文本协议判定（流式及非流式、token/按次渠道价）。会转为 Chat Completions 的账号、未规范化的 legacy messages/functions/prompt 等别名保持原结算路径，不获得 receipt。已准入请求在发送前仍核实最终账号/请求范围。请求在每次真实发送前，锁定原订阅及套餐、复核状态/到期/额度，并绑定 server request、真实 attempt、用户、key、group、account、原 SubscriptionID/PackageEntitlementID、原请求 hash 与版本化价格快照。原有 request/attempt/evidence/billing links 和 legacy fingerprint 保持原职责；不建立第二套准入 attempt 或账本。

撤销先提交则不能发送；准入先提交且随后取得真实 usage，可以按原价格向原对象结算。撤销阻止新的 failover、换号和 compact 兼容重试。曾经有未知用量的尝试仍保持 unknown，后续一次有证据的结算不能推断先前尝试免费。一个私有 server request 只允许一份真实 usage 与命令；不同请求即使使用相同客户端 request header 仍分别结算。

自然到期、并发耗尽、错误归属、缺少/损坏准入证明不获得撤销豁免。首个撤销事件保存先前状态和有效额度，后续 Restore/reset/续套餐不会隐藏之前的耗尽或改绑新套餐。精确的已完成 replay 可跨撤销/到期复核，不能再次增加计数器。

计数口径保持：普通订阅 daily/weekly/monthly 各加 SubscriptionCost；套餐 request_used 加 1、amount_used_usd 加 BilledCost、token_used 加四类 token（input/output/cache creation/cache read）。旧精度规则及金额 fingerprint 不变，完整 payload 另有版本化 SHA-256 绑定。

## 持久化与恢复

迁移 280 新建 receipt、不可变业务生命周期事件和 outbox 三张表。没有历史回填、推测用量或历史重计费。

价格快照显式保存 ResolvedPricing 的两个私有字段、区间/时段/层级价、计费候选及采用资格、个人/组/账号倍率、附加费、Free Fast 和长上下文策略。账号统计只保存所需定价规则。原始 prompt、响应正文和凭据不进入快照。

取得真实用量后，按 subscription→entitlement→request 的顺序加锁，在一个事务中同步保存原 usage、原完整命令和未结算显示投影，提交后命令才可被恢复 worker 领取，再执行 Apply。receipt 路径只有这一投影写入方，不再进入旧的异步尾部 writer。首期路径同步完成这一步和 Apply，故不依赖内存 usage worker 队列。进程中断后，生产构造器启动的有界恢复 worker 只领取原命令并重入原 Apply；不重发、不读取新价格。六计数器、dedup、billing link、usage settled 投影和 outbox completion 在同一事务提交；失租和任一 DB 错误均回滚计费事务。

数据库无法保存观察时不伪标 known，也不扣费。未识别的响应计费模型、未知 tier 或输出中的媒体/工具用量保留已观测数字为 pending，不能自动恢复成免费账单。输入文件、音频、图片、工具及非原生文本路径不获得本批撤销例外。原生文本 reasoning/compaction 历史项允许进入既有兼容重试，但每次实际发送重新准入。已观测的音频明细和非文本输出类型跨 SSE progressive/terminal、EOF、buffered JSON 及 passthrough 保留；即使 aggregate tokens 为零，也仅持久化 pending 证据，不生成价格命令。旧的可计费用量判据和媒体/工具价格字段不变。

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


## 原提交 88cdf339 的隔离证据（历史）

完整运行日志保存在执行环境 `/workspace/p1-inflight-evidence/`，核心 HTTP RED/GREEN 与组合回归已压缩归档在 [evidence 清单](./evidence/log-manifest.json)。清单分别记录解压后与压缩文件的 SHA-256，供核对原始字节；新 CI 步骤会上传 `subscription-inflight.log`，不能把静态审查当作运行闭环。

| 证据 | 运行基线与结果 | 范围 |
| --- | --- | --- |
| 初始契约 RED | `fdec971d`，12 场中 8 RED / 4 正向对照通过 | 撤销仍发送、撤销后已准入结算被拒、倍率未冻结 |
| compact 统计 RED | `fb2c9747` + 修复前单文件 overlay，完整迁移 406 | 实际 `gpt-5.5` 出站，客户按 `gpt-5.1`，统计价丢失导致 `account_stats_cost=NULL` |
| 原提交在途回归 | `6ae024e6` + 本批源码，56/56，完整迁移 406，`-race` | HTTP/auth/真实发送/usage/Apply/六计数器/proof/用户与管理员 API、锁等待、冻结价格及持久恢复 |
| 组合旧计数器回归 | `fb2c9747` + 本批源码，42 个叶子场景，完整迁移 406，`-race` | 普通/套餐、幂等、并发、reset、回滚与展示 |
| 冻结绑定/兼容回归 | `fdec971d` + 本批源码，4 + 36 场，完整迁移 405，`-race` | 复用合入 #360 前证据；该批后续只改变 scoped 定价及测试等待，不改变这些旧调用方的绑定/Apply 路径 |
| Live 异步测试等待 | 原断言 100 次有 1 次竞态；修正等待后 100 次通过 | 仅修测试等待 Apply 后的实际释放，保留 calls=2、release=1，不改 Live 生产行为 |

原提交在途日志 SHA-256：`ac396fa68b8c4b49b1550d866bfd4699ec1af03878b319896481f1027cb9d33b`。有效 compact RED 日志：`d145b99e703f7f5af8d7519357c5ecce2c7b3d6b1ec08a74097282d0dba9a7a6`。更早的无默认价格夹具失败、磁盘不足、尚未等待 lease 释放的恢复测试失败均保留为诊断，不计为业务 RED 或最终通过。

原提交规格审查由 `/root/p1_spec_review` 只读完成，限定 PASS，无未闭合 P1/P2；未独立重跑。受审 27 个非文档变更文件的[源码清单](./reviewed-source-sha256.txt)摘要为 `e87c8b748d3e1981a0d8b8b21884d69e5a3d10a4d985e5d83dcba139d621d179`。随后由 `/root/p1_quality_review` 独立只读完成质量审查，PASS，无新增 P1/P2；未独立运行测试。两项审查依次执行，源码清单完全匹配。8 项服务器门禁全部通过，命令、时间及原始日志哈希见 [门禁记录](./evidence/gates.json)：完整测试、unit tags、ledger integration、完整构建、Fork、带标签增量 lint、文档链接及 diff 检查。ledger integration 有 461 个 PASS 节点；3 个 browser fixture/子进程 helper 按既有条件跳过。

## 交付边界与回退

生产构造器接入恢复 worker 的行为仅在隔离 fixture 中运行验证，不能据此声称生产已启用。崩溃场景使用确定性故障注入；失租场景验证 CAS 回滚，不声称实际进程 kill 或所有多 worker 调度组合已覆盖。

迁移 280 为增量表及索引，旧写入方不使用这些表。源码回退会停止新 receipt/outbox 的生成与自动恢复，已有待结算命令必须保留并交由主线程协调；不删表、不重算历史、不据无证据窗口补扣。权限放宽、计数不一致或迁移错误应停止发布并协调审查过的源码回退。

合并、部署、生产健康检查及用户本地浏览器三身份验收由主线程/用户后续协调；本批没有执行，也不以隔离 HTTP 测试替代。


## 后续纯前端基线组合

#362 的 merge `6ae024e6`（父 `fb2c9747` + `df28f49d`）仅改变认证前端、测试及归档；`backend/`、迁移、CI 与脚本相对旧基线没有差异。本批 27 个受审源码/CI/Fork 检查文件合入后逐一哈希相同，因此按主线程授权复用此前组合证据，不把旧计数器与绑定证据写成新基线重跑。

新基线补跑认证组合 6 文件、211/211 测试通过。PR #362 的准确源 head `df28f49d0c3c355a622cc753055f4b63da4e3e9e` 六项 CI 全为 SUCCESS；这是来源证据，与本批后续 CI 分列。完整 `make test` 在 `fb2c9747` + 本批源码通过（前端 502 文件/3680 测试 PASS，3 文件/8 测试既有 skip）；后续完整构建、Fork 与相关 integration 使用合并后的工作树。

纯前端基线合入后的增量规格、质量复核再次依次 PASS。最终增量 lint 清理仅删除未使用的测试 helper、简化等价字段访问；两位审查者再次依次只读确认其余 26 文件不变、断言及逻辑不变。清理后在 `6ae024e6` 基线上重跑最终 56/406/race，11:52:39 UTC 完成，见 [最终运行记录](./evidence/commit-validation.json)。

## 根审修复及最新主线组合

原提交 `88cdf339` 虽然 CI 全绿，但根审发现范围判断过早、锁等待后到期复核缺失、音频/工具输出证据丢失和模型大小写索引不一致。此提交不能作为修后验收证据。本批以真实 HTTP + 完整 PG 迁移重新复现；39 场根审夹具为 25 个有效 RED / 14 个正向对照。修复后首轮 95 场中的原取消恢复案例暴露独立的投影死锁，保留原 5 秒恢复断言。

受控真实 PG 等待图证明：恢复 Apply 持有订阅行锁，再插入 usage；旧异步 usage writer 先持有 `idx_usage_logs_request_id_api_key_unique` 冲突项，再为 `usage_logs.subscription_id` 外键请求同一订阅 KEY SHARE，产生交叉等待。恢复 PID 93 等待旧 writer PID 94 的事务 1336；旧 writer 等待恢复事务 1335。修复使 pending 投影和观察/outbox 同事务提交后才可领取，并移除 receipt 路径全部旧尾写。受控修后记录 `pending_before_apply=1, legacy_writes=0, sends=1`，pending 可见性、Apply 失败回滚、租约和唯一性断言均保留。

独立规格审查继续发现 progressive 音频、passthrough 搜索、通用工具、无 usage 的 JSON、零 aggregate buffered 读错及非法 JSON 的边界遗漏。音频/搜索组有效 19 RED / 9 正向对照；修正的通用工具夹具为 28 有效 RED；读错/JSON 组为 20 有效 RED。一次工具夹具转换错误仍发送音频，其失败日志仅作诊断，不能计作工具业务 RED。正确工具 RED 使用单文件 overlay 仅禁用新增通用输出识别，保留已修音频和 receipt 路径。

随后独立质量审查发现两处 P2：pending 投影只核 log 与 command 一致，尚未前置完整 receipt 归属；只有 SSE content_part 事件时非文本输出可能遗漏。14 场新增夹具先得到 13 有效 RED / 1 正向对照。修复将同一个完整 receipt 身份判据复用于 pending 与 Apply，另核 log.GroupID；只从有效的 content_part.added/done 保存非文本 part。冻结原对象、价格、hash 与零 balance 要求保持不变。

最终在 `8364b462` 普通 merge 组合上，完整在途 **184/184** 叶子用例、**407** 个真实迁移、`-race` 通过。日志 SHA-256 为 `20825389ecb3dfb2952d5cd0d0b61545f05c18df3fe2fbf5a63ceccb3562335e`。其中包含实际请求的选路审计与 receipt/outbox 共存断言；迁移 279→280 顺序和 checksum 正确，61 个非共享主线文件保持原字节，三个共享文件保留双方语义。`usage_log_repo*`、迁移 279 和交接快照没有本批改写。

运行与受审源码对应 [33 文件清单](./root-fixes-reviewed-source.sha256)，摘要 `82b11f37dff7084ce04db035d7428012cd977422ac02a6df5df7c5937f59ab06`。详见 [修复批证据](./evidence/root-fixes-validation.json)、[原始日志清单](./evidence/root-fixes-log-manifest.json)、[组合保全证明](./evidence/main8364-combination-proof.json)。更早 97、149、169、170 场 GREEN 是迭代证据，不能代替最终 184 场；一次共存测试误用“已撤销”展示 helper 的失败也仅为夹具诊断。

恢复重复运行入口如下，只覆盖测试入口，不覆盖任何生产文件；每轮复用原 5 秒断言。脚本仅捕获自身新建的完整迁移 PG 容器日志，并要求全部重复成功、零 PostgreSQL ERROR 和零 deadlock。

```bash
python3 docs/archive/completed-tasks/2026-10-11-p1-subscription-inflight/repeat-recovery.py --output-dir /tmp/p1-recovery --repetitions 40
```

独立规格→质量复核依次 PASS，无未闭合 P1/P2。在 82b11f37 清单对应源码上，40/40 次恢复通过，PG 零 ERROR/deadlock；旧计数器 42、绑定复现 4、兼容 36 场均在该清单源码和完整 407 迁移上重跑，合计 266 个 HTTP 叶子场景通过 `-race`。具体哈希与服务器门禁结果见修复批证据文件。旧 `88cdf339` 的 CI 只保留历史，新的准确 head 与 CI 在草稿 [PR #364](https://github.com/tqytwe/sub2api/pull/364) 记录。发布使用普通双父 merge，既保留原 PR 历史也保留主线 `8364b462`，不 rebase、不 force。Git CLI 认证失败后，经授权 GitHub 连接器读取精确 Git 对象并逐一核 SHA、完整可达性，再普通合并；连接器写入时必须与本地已测试 tree 完全一致。

本批不修改运营定价规则，不访问生产，不发付费生成，不合并或部署。后续仍需主线程针对最终准确 head 的增量复核；生产部署、健康及用户本机浏览器验收不在本批授权范围。

## 最终全量门禁兼容修复

首轮 `make test` 的旧 Images SSE strict equality 测试发现：计费 scalar 全相等，但新增订阅 evidence 指针泄漏到旧图片 parser。保留原严格断言，只在旧 byte-wrapper 复制 scalar、隔离并恢复原 evidence 指针；原生 typed parser 不变。既有测试得到真实 RED→GREEN，随后增量规格→质量依次 PASS。

该 wrapper 受审清单为 [wrapper-reviewed-source.sha256](./wrapper-reviewed-source.sha256)，摘要 `2eff1dce05d4e1a010e7a75e2e31d2fc3b2486c70ca1c452787a4e700ed8a7ad`。相对前述 82b11f37 清单仅此 wrapper 所在文件改变。独立双审核实生产旧 wrapper 仅供 Images，native normal/passthrough/buffered 直接调用 typed parser，因此此前 266/407/race 及 40 次恢复按明确调用图复用，不写成修后重跑。最终全量门禁使用修后源码；文档整理不触发重复全量测试。

随后完整后端默认测试通过（service 145.275 秒），lint 指出三个 QF1001。仅将 forward 两处和 passthrough 一处 `!(A || B)` 改为 `!A && !B`；增量规格→质量确认短路顺序、求值次数相同，反向替换哈希与前版精确一致，复用此前运行证据。该阶段清单摘要为 `1b76a4908e35d5422114669e03c4084ca15cd937ef6f6a72d978e501aa8abf84`，后续增量与最终清单见下文。

前端源码与主线逐字节相同。第一次并行全量中 503 文件通过，既有 session HTTP 文件的五个固定 30ms 连接销毁断言失败；该文件独立 26/26 通过。保留原断言和前端源码，完整前端单独复跑以排除资源负载时序；最终门禁记录分别保留原失败及复跑结果，不将初次失败改写为成功。


完整前端独立复跑最终为 504 文件、3825 测试通过，3 文件/8 测试按既有条件跳过；包含 lint、类型与 design 检查，没有改动任何前端断言。

unit-tag 全量随后发现另 8 个既有 `TestOpenAIResponsesTerminalImageExpenseStopsReplay` 严格结构断言失败：没有 receipt 的原生 Responses 返回了新增 evidence 指针。仅在 normal/passthrough 最终结果返回前、完成费用与重试判断后，无 receipt 时清理新证据字段；原标量、图片计数、SearchCount 及严格断言保持不变。定向 unit 重跑通过（2.835 秒）。lint 的相同问题显示上限为 3，第二轮另显出 response_handling 的第四处等价布尔式，按相同方式改写。

这批增量完成独立规格→质量 PASS。最新完整在途重跑 **184/184、407 迁移、race PASS**，日志 SHA-256 `28a80311e674f649e474c3671f9054aedf789ef9a1725719a53d19f5dda40503`。该轮 [33 文件清单](./receipt-scope-reviewed-source.sha256) 摘要 `de9cb070b1a63d41af2a1fad3fea5145418edc9d11cb14cca77c26f93e5bf92b`。旧兼容 82 场与恢复 40 次按 unchanged binding/Apply/recovery 路径复用，明确不是这次最终源码重跑；已准入输出证据的保留由最新 184 场补证。


带标签 lint 随后要求测试代理检查两处接口类型断言。仅改为 comma-ok guard；成功路径仍调用同一 writer，接口缺失返回显式 fixture 错误，原计数/投影断言不变，没有生产修改。适用 `projection_single_owner` 再跑 **407 迁移/race PASS**，仍为 `pending_before_apply=1 / legacy_writes=0 / sends=1`。本轮 [33 文件清单](./final-reviewed-source.sha256) 摘要 `72275ca942199f9c3a78bdd16672138ca228b2957a5a1d7de2b9a62e41dfe43b`；此前184/82/40注明测试 guard 等价复核复用，默认/unit/ledger/build/Fork构建标签均不包含此测试文件。
