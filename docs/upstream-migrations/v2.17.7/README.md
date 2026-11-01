# v2.17.7 同步批次账本

> **#399 发布后复核（2026-11-01 UTC+8）**：PR #399 已通过全部 8 项 CI 并合并为 `play/main@80131c818b1d1917c75a73bb61becc1930efd550`；Zeabur deployment `6ae66d6eea16512cf741d587` 的实际 commit SHA 一致，`finishedAt=2026-10-31T22:41:24.068Z`，状态 `RUNNING`。应用、PostgreSQL、Redis 均为 `RUNNING`，`www.jisudeng.com` 与 `/health` 均 HTTP 200（`{"status":"ok"}`）。部署后运行日志未发现 `ERROR/FATAL/PANIC`；可见 WARN 为既有配置提示（审核 API Key、trusted proxy、URL allowlist、JWT 持久化密钥和价格 fallback），不构成启动失败。未认证访问 `/api/v1/admin/accounts`、`/api/v1/admin/requests`、`/api/v1/admin/usage` 均返回 `401 application/json`。#399 无新增 SQL migration，未执行生产写入；内容审查用户管理刷新已由用户确认正常。#397 管理员已验收质量规则保存/刷新、2 模型×2 次产生同一轮 4 条结果及跨计划历史；#396 管理员已验收多组 Key、默认组切换、保存刷新和倍率明细。Priority 真实 WS 观测行、渠道运维写动作和历史账务只读归因仍待完成，`fully_migrated=false`。本段是发布后交付证据，不为文档单独触发部署。

状态：`deployed-partial / migration-incomplete`；整版仍为 `fully_migrated=false`。当前生产以 #399 精确部署记录为准。

本目标是版本功能同步，不能以 v2.17.7 release notes 的末尾修复代替完整差异盘点。生产与上游源码的共同祖先为 `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`（上游 `0.2.5`）；上游从该点至目标 tag 有 2,155 个提交、446 个 first-parent merge，按共同祖先直接比较有 3,809 个路径变化。以上数字只界定审查范围，不表示需要照搬这些提交或文件。Fork 自有实现必须逐项保留/映射；仅纳入相关产品能力和必要安全兼容修复。

## 来源锁定

| 字段 | 值 |
|---|---|
| 来源仓库 | `ranxi2001/sub2api` |
| Release | `v2.17.7` |
| Tag object | `af9a09cf92c1cbfdcf793862122d46a5a29a6da1` |
| Release commit | `9620d315f72a7dbf04fa10fca02970e1e410805e` |
| 上游共同祖先 | `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a` (`0.2.5`) |
| 历史生产分析基线 | `origin/play/main@21dd9af2239c2c7f8259e41e31e21156131622b4`（PR #392；不代表当前线上） |
| 当前已核实生产 | `play/main@80131c818b1d1917c75a73bb61becc1930efd550`（PR #399；Zeabur deployment `6ae66d6eea16512cf741d587`；2026-11-01复核） |
| fully migrated | `false` |

## 已发布批次与整版状态

| 检查项 | 当前证据 | 结论 |
|---|---|---|
| 合并/部署 | #399 SHA `80131c818b1d1917c75a73bb61becc1930efd550`；Zeabur deployment `6ae66d6eea16512cf741d587` SHA一致、RUNNING | 当前生产，详情见首段 |
| 运行态/CI | 主站/API health200；#399构建完成，运行日志ERROR/FATAL/PANIC为0；既有配置 WARN 见首段 | 不替代管理员功能验收或完整长期日志审计 |
| 管理员闭环 | 内容审查列表刷新由用户再次确认正常；Usage反馈正常；#397质量多模型/同轮4条结果/历史和#396 Key/倍率均已通过本机验收 | Priority真实WS观测、渠道运维写动作及其他管理动作仍待完成 |
| 数据库生产账本 | 431条ledger，424个当前SQL均已应用且checksum一致，7条历史；本批无迁移 | 已部署迁移核验；21,433条历史结算待逐笔归因 |
| #397 质量发布前预检 | 部署前只读：规则1条，enabled1、observe_only1，多模型0、重复probe0、max_samples1、超过候选上限0；显式ROLLBACK | 部署没有自动把现有规则升级为多模型/高频检测；后续组合验收已由管理员完成 |
| 当前质量批次 | PR #398 已合并部署：质量历史查询、WebSocket优先调度归因及空 delta 语义修复；继承#397的多模型/重复probe与整轮持久化 | 生产运行核验和管理员本机质量组合验收通过；Priority真实WS观测行仍待核对，见开发清单第5节 |
| 整版差异 | 446个merge初步分流：122 reviewed、324 triaged；仍有scope/evidence/split及阻断项 | `fully_migrated=false` |

#393已部署质量runtime/ownership/Jev/模板/预设/跨计划历史、滚动Usage和moderation；#394修好moderation列表；#395已部署AutoRestore/unknown、凭证安全历史/structured invalid_grant及单模型Priority round；#396已部署API Key多组；#397已部署质量多模型/重复probe。后续只补实际缺口，不重开这些基础开发。历史settlement、权益审计、完整部署窗口和上游#492全文/跨用户读取/无TTL阻断继续独立跟踪；不补扣、不改历史数据、不新增正文采集。

## 必审正式变更

- 质量运维：凭据错误安全展示、账号保存保留服务端隔离状态、质量规则归属与恢复/删除确认。
- GPT-6.1 Sol：长上下文计费阈值、OpenAI OAuth/Setup Token 豁免边界及最终模型映射。
- 用量时间范围：精确滚动 24 小时窗口、筛选快照、清理日期范围和非 UTC 聚合回查。
- 内容审查用户管理：管理员独立解除禁用入口、原因确认、写入前后状态回读，不改变白名单或其他安全限制。

## 关键上游 PR 差异结论

| 上游 PR | 当前结论 | 剩余边界 |
|---|---|---|
| #572 优先调度 | hard-stop、attempt持久化、分位数/惩罚时钟及多模型quality round已部署 | 本批已接入原生WS首个解析确认语义事件的attempt观测，单测及隔离PostgreSQL回归通过；管理员验收调度与质量配置联动，部署后需核对真实观测行 |
| #573 渠道运维 | Fork适配已部署，保留282迁移/CAS/审计；生产质量统计此前固定返回 `unsupported_scope` | 已部署复用存储质量轮次的查询与完整性判定；存储/API单测、隔离PostgreSQL 18.1集成和 PR CI 已通过，管理员页面验收待完成；无新增迁移 |
| #574 凭证异常 | guard v2/reauth/writer-reader/API/UI及334已部署；#395修历史安全详情/structured invalid_grant | timeout分类和管理员动作逐项复核，不重造worker或迁移 |
| #575/#576 质量/Jev | runtime、ownership、Jev、模板/预设、跨计划历史、AutoRestore及多模型/重复probe整轮闭环已部署 | 管理员验收；模型/分组动作、额外judge和live替换后续定范围 |
| #577 GPT-6.1 Sol | Fork等价/部分等价 | 生产账务样本只读核对待办 |
| #579 滚动24小时 | #393已部署API/UI，用户反馈正常 | 精确窗口/导出/日期清理本机数据验收待补 |
| #580 moderation | #393部署296/298/301及CAS/generation/exemption/owned unban，#394修好列表且已验收 | 真实写动作与审计待验收，不新增重复入口 |
| #581 版本号 | 保留Fork `0.2.14` | 不声称已切换上游 `2.17.7` |
| request capture / support / harvest | 本批保持阻断 | 先明确范围/隐私/保留合同 |
| security penalty | 隐私阻断 | 禁止原样新增明文全文副本、跨用户读取与无TTL存储 |

## 后端差异审查

| 上游能力 | 当前生产证据 | 本批/后续验证 |
|---|---|---|
| 质量运维 | #393/#395基础能力及#397多模型/重复probe、整轮保留/历史和逐模型Priority归因均已部署 | 管理员本机组合验收已通过；真实恢复动作、分组/模型动作和live替换仍待完成 |
| GPT-6.1 Sol计费 | 272000阈值、OAuth/Setup Token不超过500000豁免、缓存token、最终模型和API key边界已有代码/测试 | 只读抽查长上下文、余额/冻结价格；不补扣未知请求 |
| 管理端滚动24小时 | #393已部署精确ISO窗口、同快照筛选/分页/导出及日历清理保护，已有时区/DST测试 | 精确数据/导出/清理本机验收待补，不重做API/UI |
| 内容审查用户管理 | 296/298/301、state/hits/actions与索引已核验；UID查找/CAS/generation/reset/exemption/原因/owned unban已有实现；列表刷新已验收 | 其他管理员真实写动作、审计和普通用户拒绝按受影响范围补证 |

当前生产以首段 #399 部署证据为准；下方链接的历史候选和旧表缺失记录不作为新开发任务。

## 当前集成规则

1. 逐项比较上游实现、生产已有实现和 Fork 定制，不执行整树合并。
2. 已部署迁移不可改写；迁移必须按完整文件名、checksum 和 schema 依赖审查。
3. 普通用户请求台账入口保持移除，任何新页面必须有真实管理员权限和后端合同。
4. 代码、前端、数据库、文档和测试累计为一个生产发布批次；未通过统一审查前不得合并或部署。

## 下一批次执行队列

1. **#397 生产验收（已完成）**：管理员在 `/admin/account-quality` 以低成本测试规则和 `observe_only` 跑2模型×2次，确认保存刷新、同一轮4条结果和跨计划历史均正常。
2. **#396 管理员验收（已完成）**：普通用户多组绑定、默认组切换、保存刷新和管理员倍率明细均正常；普通用户入口/API 拒绝继续按既有权限门禁单独保留。
3. **账务历史归因**：对21,433条 `settlement_pending` 先做脱敏只读分层；唯一有 link 的记录缺 dedup/归档证明，usage 未 settled 且 entitlement ID 不一致，需由业务/财务确认，不补扣、不改状态。
4. **P0 渠道/调度验收**：对照 #573、#572 逐项核对管理员保存、CAS、审计、普通用户拒绝及调度 hard-stop/unknown 语义；不重复已部署的运行时基础。
5. **整版收口**：关闭446范围中的 split/evidence/product-scope；原生WS观测、质量分组动作/live替换、凭证timeout和管理员动作按边界分批推进。正文/harvest/#492及新运行时范围继续阻断。

## 全版本差异矩阵

| 工作包 | 负责人 | 当前状态/剩余门槛 |
|---|---|---|
| 来源/覆盖 | 集成负责人 | 446个SHA唯一，122 reviewed/324 triaged；继续关闭split/范围/证据项 |
| 后端/API | 后端负责人 | #397多模型/整轮与模型归因已部署；质量组合验收已完成，原生WS真实观测和优先调度动作按清单推进 |
| 前端 | 前端负责人 | #397沿用真实AccountQualityView/组件/API；生产管理员多模型组合验收已完成，其他受影响页面按清单推进 |
| 数据库/计费 | 数据库/计费负责人 | 431 ledger/424当前SQL及337/338已核验；21,433条账务待归因，含一条缺失精确dedup且权益关联不一致的记录 |
| 集成/发布 | 主线程/发布负责人 | #397已合并部署；下一批累计改动后再提一个PR并完整监督CI/部署，不为文档单独发布 |

具体功能域采用[功能矩阵](./feature-matrix.md)和[开发清单](./DEVELOPMENT_CHECKLIST.md)，不再把旧候选条目复制为新开发工作。保留Fork的066/070定时测试、outbox、273模型范围及已部署282/334/337/338；同名异内容迁移不可覆盖，不按上游数字前缀直接搬运。request-capture与Fork自有管理员请求台账不是同一功能，额外运行时/页面仍按范围合同评审。

## 生产交接验收清单

### 管理员现场检查

使用用户本机浏览器和现有管理员账号，不在交接文档中记录账号或令牌。内容审查列表加载/刷新已由用户再次确认正常，不重复验收；下表写动作仍各自待办：

| 检查 | 操作与通过证据 | 状态 |
|---|---|---|
| 渠道运维入口/权限 | 管理员打开 `/admin/channels/operations`，页面可加载；普通用户侧栏没有该入口，直达路由及 API 被拒绝 | 等管理员现场确认 |
| 渠道运维读写 | 查询渠道目录和运维快照；对一项无生产影响的配置做经批准的测试变更，保存后刷新仍一致；冲突 revision 返回可理解错误 | 等管理员现场确认；不要用真实流量开关做未经批准的试验 |
| 审计 | 对上述测试变更确认审计记录含操作者、目标 ID、结果和版本，不含凭证、token、原始上游响应 | 等管理员现场确认 |
| 优先调度 | 管理员打开 `/admin/priority-scheduling`；确认读取/保存/刷新一致、状态明确；候选不得绕过硬准入；无历史/质量数据时显示 unknown/unavailable 而非伪造通过 | 等管理员现场确认 |
| 质量多模型轮次 | 管理员在 `/admin/account-quality` 用低成本规则设 `observe_only`，跑2模型×2次；保存刷新后确认同一轮4条结果、跨计划历史和Priority逐模型归因 | 已由管理员确认保存刷新、同轮4条结果及跨计划历史；Priority真实观测仍待核对 |
| 内容审查快捷启用 | 仅对获准的测试对象核对管理员可见、普通用户不可见、启用前确认和状态回读；不得改变白名单或其他安全限制 | 等管理员现场确认 |
| 请求台账边界 | #384管理员专属入口已经用户验收；本批受影响时回归直达页面/API拒绝 | 已完成旧批验收，不重复列旧用户验收门槛 |

### 日志与数据库跟踪

上表中的生产SHA、迁移账本和结算归因更新至#397后复核；历史日卡权益、282锁耗时及完整部署日志仍为较早采样，未在本轮重新读取，不据旧数量推断当前状态。历史风险尚未关闭。

| 检查 | 必须取得的证据 | 状态 |
|---|---|---|
| Zeabur 应用/数据库日志 | #397实际SHA/RUNNING及主站/API health200已核验；启动日志74行无ERROR/WARN/FATAL/PANIC | 有限采样通过，完整部署窗口与历史PostgreSQL恢复外因仍待补证 |
| 生产迁移账本 | #397只读复核431 ledger，424当前SQL无缺失/checksum差异，7历史；337/338均已应用，338表/主键/2外键/索引存在，invalid indexes=0，ROLLBACK | 已核验；本批无schema迁移 |
| 282 schema 结果 | 两张目标表、`channels.operations_revision NOT NULL DEFAULT 0`、索引、唯一/检查约束及外键均存在；upstream/member 行数为 0/0，revision 空值/负数和成员孤儿均为 0。只读检查未改业务数据；迁移锁等待/耗时未从完整启动日志证实 | 表结构/不变量已核验；运行时锁/耗时待日志 |
| 历史日卡权益/账务 | 日卡相关历史表：replay 4,136、hold 5,095（captured 4,135 / released 960）、entitlement 21（active 1 / exhausted 5 / expired 15）。captured hold 缺 replay=0、completed replay 缺完成时间=0、pending/forwarding/retryable=0；2,547 replay 有 usage log 对应。唯一 `active` 但已过期的记录来源为 `redeem_code`，关联订阅已 expired，无 payment order link；未发现同一账户组的当前有效套餐权益，当前代码树也未读取旧表 | 未见当前有效权益或请求放行风险；历史兑换审计语义由业务负责人确认，不改状态、不清理 |
| 当前套餐权益状态卫生 | 生产有 4 条 `subscription_package_entitlements.status=active` 但 `expires_at` 已过期的记录；均无当前有效订阅、其他有效套餐权益或有效历史日卡权益 | 代码按时间动态判为 expired，准入 SQL 同时要求 `expires_at > now()`；未见当前权益放行风险，不自动修复状态 |
| 请求/错误日志 | Zeabur runtime API 只返回最近 100 行，当前应用样本无错误级别记录，较早样本出现 11 条 ERROR/WARN marker；完整部署前后 5xx、关键 admin API、DB、scheduler 和用户请求影响不可由此证明 | 部分采样已核验；完整窗口和业务影响待办 |
| Gateway 结算与用量归因 | 10:23 UTC 只读聚合：19,538 条 `settlement_pending`；19,252 >1h、19,044 >24h、14,344 >7d、never reconciled=2。19,537 条没有 billing link；状态组合主要为 failed/usage_unknown 17,073、cancelled/usage_unknown 2,250、interrupted/usage_unknown 190、succeeded/usage_unknown 18，另有 succeeded/known 1 和新鲜 inflight/pending 5。剩余一条 link 未验证、未证明扣费；subscription outbox 81 全 completed、pending receipt=0。`settlement_state` 是请求账本状态，不能代替扣款证明 | 高优先级待业务/财务脱敏抽样，关联 usage log、dedup/archive proof、余额流水和订阅扣额；不得批量修状态或补扣 |
| v2.17.7 #492 security penalty 明文保留审查 | `293_security_penalties.sql` 新增 `security_penalty_events.full_prompt TEXT`；writer 明文保存至多 100,000 runes，无自动 TTL/cleanup；admin detail 可跨用户取全文，用户只能取自己的。当前 Fork 的 `prompt_audit_events` 已存最多 65,536 runes 明文，也只有人工删除、无 TTL，因此原样迁入会额外保存副本并扩大读取范围。#293 DDL/运行时不依赖 #519 recovery，不应把两者捆绑 | 阻断原样迁入；先确认全文是否必须持久化、管理员最小读取范围、保留期限/自动清理和审计合同；只读源码证据，不读取生产 prompt 正文 |
| GPT-6.1 Sol 计费与用量 | 核对部署前后长上下文、余额、冻结价格和订阅样本；避免将 unknown usage 当作实际扣款 | 待业务数据只读抽样 |

生产检查必须由具备相应权限的管理员/数据库维护者执行并把脱敏结果回填本表。禁止将生产 DSN、token、用户凭据或完整请求正文写入 issue、日志摘录或仓库。无证据的条目保持“待办”，不要用 `/health` 200、部署成功或单元测试代替。

## 历史交付证据

已删除本页重复的旧候选任务、过期生产状态和同日采样叙述。原始只读审计、回滚和部署证据保存在 [#399 部署时的账本快照](https://github.com/tqytwe/sub2api/blob/80131c818b1d1917c75a73bb61becc1930efd550/docs/upstream-migrations/v2.17.7/README.md) 及 [PR #393](https://github.com/tqytwe/sub2api/pull/393)、[#394](https://github.com/tqytwe/sub2api/pull/394)、[#395](https://github.com/tqytwe/sub2api/pull/395)、[#396](https://github.com/tqytwe/sub2api/pull/396)、[#397](https://github.com/tqytwe/sub2api/pull/397)、[#398](https://github.com/tqytwe/sub2api/pull/398)、[#399](https://github.com/tqytwe/sub2api/pull/399)。历史数字不代表当前线上采样；迁移与回滚文件未删除。
