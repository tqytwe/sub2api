# v2.17.7 同步批次账本

> **#398 发布后复核（2026-11-01 UTC+8）**：PR head 已普通 merge 为 `play/main@db76a7e2a0a8c24818efad1fad6623fee28ab706`；Zeabur deployment `6ae65455ea16512cf741d4c8` 的 commit SHA 一致，`finishedAt=2026-10-31T20:53:58.935Z`，状态 `RUNNING`。构建日志以 `build completed` 结束，无失败/failed/panic；应用、PostgreSQL、Redis 服务均为 `RUNNING`，域名为 `PROVISIONED`，首页和 `/health` 均 HTTP 200（`{"status":"ok"}`）。部署后运行日志筛查为 ERROR/FATAL/PANIC 0 条，只有 1 条既有 `content_moderation.skip_no_audit_api_keys` WARN；该提示表示审核 API Key 未配置，不是本批启动失败。未认证访问质量计划/历史、渠道运维、内容审查用户、请求台账和管理员 Usage 接口均返回 `401 application/json`。此次提交不含新的 SQL migration，沿用 #397 已核验的 431 条 migration ledger（424 个当前 SQL checksum 匹配、7 条历史记录）；本批生产数据库未写入。管理员多模型质量轮次、Key/倍率和其他业务动作仍待现场验收，`fully_migrated=false`。本段是发布后交付证据，不为文档单独触发部署。

> **#397 当前生产状态（2026-11-01 UTC+8）**：PR head `70524682fd0b27de9fb7fd2dbc474906a86a362f` 已通过9项CI并合并为 `play/main@f7193d639980840ae9fad66376355f4243d82a35`；Zeabur deployment `6ae634e9ea16512cf741d3ca` SHA一致、RUNNING，主站/API health 200。生产 migration ledger 431条，424个当前SQL checksum匹配、7条历史记录，本批无schema迁移。用户确认内容审查用户管理刷新正常；#397多模型/重复probe管理员验收、#396 Key/倍率验收和账务历史归因仍待完成。账务只读聚合与匹配证据见[开发清单](./DEVELOPMENT_CHECKLIST.md)；无生产写操作。`fully_migrated=false`，正文采集/harvest和#492全文审计继续阻断。

> **#396 历史发布复核（由 #397 覆盖）**：新 head `f09b7230fce899370db56f4e98350e15ed1ee8a6` 的9项CI全部通过，已普通merge至 `play/main@73f9ba38b4d61ad5a0e0ac05ab2004d4c9775444`，与受测head源码一致。Zeabur deployment `6ae61c6cea16512cf741d279` 的SHA一致，构建镜像导出完成、`finishedAt=2026-10-31T16:56:01.268Z`、RUNNING，主站/API health200。数据库只读复核431条ledger，424个当前SQL全部匹配、7条历史；338表/主键/2外键/索引存在，invalid indexes=0，事务ROLLBACK。最近100行应用日志ERROR/FATAL/PANIC为0，WARN为缺少审核API Key；CLI有限窗口不证明全启动日志无异常。Keys接口一次探测超时后主站/API重查均401，其余未认证接口401。技术部署核验通过，本批Key与管理员倍率的本机浏览器验收仍待完成；内容审查刷新已由用户确认，不重验。完整正文/harvest继续阻断，`fully_migrated=false`。详细证据和剩余验收见 [PR #396](https://github.com/tqytwe/sub2api/pull/396)。本段为发布后本地补记，随下一批累计提交，不为文档再次部署。

> **#395 历史部署记录（由首段#396覆盖）**：生产为 `play/main@aea9be1e6508c5c319cc763c5fe058bc8d4b4aed`（PR [#395](https://github.com/tqytwe/sub2api/pull/395)），Zeabur deployment `6ae5dc8eea16512cf741cda0` 状态 `RUNNING`、实际 SHA 一致；主站和 API `/health` 均 HTTP 200。用户再次确认内容审查用户管理刷新正常，原 internal error 故障验收已通过。#395 的质量恢复、优先调度 quality round 和凭证修复已部署，不重复开发。
> PR [#396](https://github.com/tqytwe/sub2api/pull/396) 对应上游 #479 API Key多组，现已合并、部署并完成338表/约束/索引核验；当时“338未应用、候选待CI”的预发布记录已由首段生产证据覆盖。Key/倍率本机验收仍待完成。

当前生产已包含质量多模型/重复probe、整轮持久化/历史和Priority逐模型归因，并由 PR #398 补充质量历史查询、WebSocket 优先调度归因和空 delta 语义修复。开发分支 `codex/quality-multiprobe-20261101` 是已合并的交付分支，不是候选；剩真实管理员验收。下一批按下方工作矩阵和[开发清单](./DEVELOPMENT_CHECKLIST.md)继续，不单独发布文档更新。以下历史段落不代表当前状态。

> **2026-10-31 #394 历史修复部署复核（UTC+8，当前生产见首段）**：PR [#394](https://github.com/tqytwe/sub2api/pull/394)
> 全部 9 项 CI 检查通过，修复提交 `d99f2c8a2` 已合并至
> `play/main@3da3cb0e953fe751b50a2f6f446e8c24a62c80b2`。
> Zeabur deployment `6ae5a1eaea16512cf741c89b` 的实际 SHA 与合并 SHA 一致，
> `finishedAt=2026-10-31T08:13:36.402Z`、当前状态 `RUNNING`；构建日志有镜像导出完成证据。
> 应用、PostgreSQL、Redis 均为 `RUNNING`，首页和 `/health` HTTP 200，
> 未认证访问 `/api/v1/admin/risk-control/users` HTTP 401。
> 用户在本机刷新后确认“加载正常，没有 internal error”；18:17:56–18:18:09 的
> 5 次该接口请求全部 HTTP 200，18:17:40–18:18:43 的应用最近 100 行无 ERROR 或原 SQL 参数错误。
> PostgreSQL 最近 100 行中，部署完成之后的 51 行无 ERROR/FATAL/PANIC；旧参数错误发生在部署前。
> 较早应用采样有 1 条上游转发 `context canceled`，不归为本次 SQL 缺陷，不能据有限窗口宣称全站无异常。
> 本次未新增数据库迁移或执行数据修复。补充 SQL 只读复核最初超时；关闭 psql 分页器并禁止交互密码提示后，
> 在 `BEGIN READ ONLY`、7 秒 statement timeout 和 2 秒 lock timeout 下成功完成实际参数推断的
> PREPARE/EXECUTE：7 种筛选的统计与列表一致（all/enforced 为 2/2，其他筛选均 0/0），
> 不存在用户的定向统计为 0。两条 PREPARE 均已 DEALLOCATE，已取得显式 `ROLLBACK` 回执；CLI 返回 0。
> 此次列表刷新故障已有生产请求与用户本机确认；整版同步仍保持 `fully_migrated=false`，
> 游客/普通用户完整浏览器验收、既有批次其他业务验收和数据库独立跟进项不因此关闭。
> 本段为发布后本地交付补记，待后续累计交付纳入仓库，不单独触发生产部署。

> **2026-10-31 故障定位历史（内容审查用户列表；由上方复核覆盖）**：PR [#393](https://github.com/tqytwe/sub2api/pull/393)
> 已部署到 `play/main@e11c3166945d01012a104b5f5157cb34eb98b611`，Zeabur deployment
> `6ae58008ea16512cf741c5b4` 已完成构建并运行；此前“候选未部署/生产 #392”的描述是历史记录。
> 管理员反馈 Usage 正常；内容审查用户管理刷新返回 500，15:08:31（UTC+8）应用日志定位为
> `count content moderation users: pq: could not determine data type of parameter $1`。
> 原因是总数 SQL 使用 `$3/$4` 却绑定了未使用的 `$1/$2` 时间窗口参数。
> 修复分支 `fix/moderation-users-count-20261031` 仅修正统计参数并复用同一筛选条件；不改数据库、前端、权限或审核处罚规则。
> 原查询已在生产只读事务内通过 PREPARE 复现并显式 ROLLBACK；回归测试先失败、修复后通过。
> 完整测试、构建、PR CI、修复部署和管理员刷新复验仍在进行，**不能宣称生产缺陷已解决**。
> 整版同步仍保持 `fully_migrated=false`；本次修复不改变正文采集/harvest 的既有阻断边界。

## #394 后工作队列（历史，已由 #395 部署覆盖）

- 生产基线为上述 #394；#393 的质量/Usage/moderation/凭证能力已上线，不沿用下方“候选未部署”的历史叙述。
- 数据库新证据：生产 ledger **429** 条、当前源码 SQL **422** 个，缺失 **0**、checksum 差异 **0**，历史额外记录 **7**；296/298/301/320/334 与目标表/索引均在；只读事务显式 ROLLBACK。
- 账务独立只读观察：本轮 `settlement_pending=21,326`，其中部署前21,284、部署后42；21,325条无精确billing link。按执行状态为failed18,682、cancelled2,426、interrupted195、succeeded19、inflight4；usage_unknown21,321。`settled`但存在未验证link的计数为0。此为聚合证据，不足以判断欠扣/重复扣费；历史归因继续待办，未补扣、未改状态、未清理，事务ROLLBACK。
- 本地累计分支 `codex/v2177-post394-20261031`：AutoRestore 显式配置/执行、质量历史 unknown、不安全历史 ProbeDetail 读出、structured invalid_grant 分类、Priority quality round 与337稳定配置 revision 已完成；后端完整测试/unit/lint/build、前端全量测试/类型/lint/build、Fork integrity 和文档检查通过，尚未提交/部署，真实 PostgreSQL 集成执行待 PR CI。
- 本地尝试编译并执行 PostgreSQL integration suite 时，Docker 拉取 `postgres` 镜像因 Docker Hub 网络超时未启动容器；该失败不代表测试逻辑通过，必须由 PR CI 的 disposable PostgreSQL 作最终执行证据。
- 优先调度质量信号为本批后端实际缺口：两种reader没有填质量字段；运行器updated_at每次claim/finish变化，不能直接抄上游时间过滤。实现需独立稳定配置时间、quality有效期与完整round聚合，不引入额外provider/区域运行时。
- 已取消重复开发：审核记录行独立启用用户、跨计划历史、真实ownership恢复、最终HTTP/WS准入已存在；moderation相关现有前端/API回归重跑42/42。
- 上述本地修复将统一检查、审查、一个PR和一次生产发布；发布后立即查Zeabur实际SHA、日志、迁移、权限/API，再做受影响管理员本机验收。不能用本段替代尚未执行的发布。
- 后续剩余范围见 [开发清单](./DEVELOPMENT_CHECKLIST.md) 与 [功能矩阵](./feature-matrix.md)。446条范围准确不等于446条行为已迁移；保持`fully_migrated=false`。

下方明确标注“历史”的段落只保留审计过程；当前状态以首段及当前表为准。

状态：`deployed-partial / migration-incomplete`。截至#397已部署部分同步能力，`fully_migrated=false`继续有效。当前生产以首段及下表为准；文末10月29日采样仅保留审计历史，不覆盖新部署。

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
| 当前已核实生产 | `play/main@db76a7e2a0a8c24818efad1fad6623fee28ab706`（PR #398；Zeabur deployment `6ae65455ea16512cf741d4c8`；2026-11-01复核） |
| fully migrated | `false` |

## 已发布批次与整版状态

| 检查项 | 当前证据 | 结论 |
|---|---|---|
| 合并/部署 | #398 merge `db76a7e2a0a8c24818efad1fad6623fee28ab706`；Zeabur deployment `6ae65455ea16512cf741d4c8` SHA一致、RUNNING | 当前生产，详情见首段 |
| 运行态/CI | 主站/API health200；#398构建完成，运行日志ERROR/FATAL/PANIC为0，仅1条既有审核API Key提示 | 不替代管理员功能验收或完整长期日志审计 |
| 管理员闭环 | 内容审查列表刷新由用户再次确认正常；Usage反馈正常 | #397质量多probe、#396 Key/倍率写动作验收及其他管理动作仍待完成 |
| 数据库生产账本 | 431条ledger，424个当前SQL均已应用且checksum一致，7条历史；本批无迁移 | 已部署迁移核验；21,433条历史结算待逐笔归因 |
| #397 质量发布前预检 | 部署前只读：规则1条，enabled1、observe_only1，多模型0、重复probe0、max_samples1、超过候选上限0；显式ROLLBACK | 部署没有自动把现有规则升级为多模型/高频检测；管理员组合验收仍待完成 |
| 当前质量批次 | PR #398 已合并部署：质量历史查询、WebSocket优先调度归因及空 delta 语义修复；继承#397的多模型/重复probe与整轮持久化 | 生产运行核验通过；管理员本机组合验收待完成，见开发清单第5节 |
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
| #573 渠道运维 | Fork适配已部署，保留282迁移/CAS/审计；生产质量统计此前固定返回 `unsupported_scope` | 当前候选复用已存质量轮次实现查询与完整性判定；存储/API单测与隔离PostgreSQL 18.1集成通过，PR CI和管理员页面验收待完成；无新增迁移 |
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
| 质量运维 | #393/#395基础能力及#397多模型/重复probe、整轮保留/历史和逐模型Priority归因均已部署 | 管理员本机组合验收、真实恢复动作验收待完成 |
| GPT-6.1 Sol计费 | 272000阈值、OAuth/Setup Token不超过500000豁免、缓存token、最终模型和API key边界已有代码/测试 | 只读抽查长上下文、余额/冻结价格；不补扣未知请求 |
| 管理端滚动24小时 | #393已部署精确ISO窗口、同快照筛选/分页/导出及日历清理保护，已有时区/DST测试 | 精确数据/导出/清理本机验收待补，不重做API/UI |
| 内容审查用户管理 | 296/298/301、state/hits/actions与索引已核验；UID查找/CAS/generation/reset/exemption/原因/owned unban已有实现；列表刷新已验收 | 其他管理员真实写动作、审计和普通用户拒绝按受影响范围补证 |

以上当前状态以#397部署证据为准；历史候选和旧表缺失记录见下方历史区，不作为新开发任务。

## 当前集成规则

1. 逐项比较上游实现、生产已有实现和 Fork 定制，不执行整树合并。
2. 已部署迁移不可改写；迁移必须按完整文件名、checksum 和 schema 依赖审查。
3. 普通用户请求台账入口保持移除，任何新页面必须有真实管理员权限和后端合同。
4. 代码、前端、数据库、文档和测试累计为一个生产发布批次；未通过统一审查前不得合并或部署。

## 下一批次执行队列

1. **#397 生产验收**：管理员在 `/admin/account-quality` 以低成本测试规则和 `observe_only` 跑2模型×2次，确认保存刷新、同轮4条结果和跨计划历史；必要时核对单模型规则语义。
2. **#396 管理员验收**：完成 API Key 多组与倍率保存/刷新/冲突回读，并确认普通用户入口/API 仍拒绝。
3. **账务历史归因**：对21,433条 `settlement_pending` 先做脱敏只读分层；唯一有 link 的记录缺 dedup/归档证明，usage 未 settled 且 entitlement ID 不一致，需由业务/财务确认，不补扣、不改状态。
4. **P0 渠道/调度验收**：对照 #573、#572 逐项核对管理员保存、CAS、审计、普通用户拒绝及调度 hard-stop/unknown 语义；不重复已部署的运行时基础。
5. **整版收口**：关闭446范围中的 split/evidence/product-scope；原生WS观测、质量分组动作/live替换、凭证timeout和管理员动作按边界分批推进。正文/harvest/#492及新运行时范围继续阻断。

## 全版本差异矩阵

| 工作包 | 负责人 | 当前状态/剩余门槛 |
|---|---|---|
| 来源/覆盖 | 集成负责人 | 446个SHA唯一，122 reviewed/324 triaged；继续关闭split/范围/证据项 |
| 后端/API | 后端负责人 | #397多模型/整轮与模型归因已部署；管理员组合验收、原生WS观测等缺口按清单推进 |
| 前端 | 前端负责人 | #397沿用真实AccountQualityView/组件/API；生产管理员多模型组合验收仍待完成 |
| 数据库/计费 | 数据库/计费负责人 | 431 ledger/424当前SQL及337/338已核验；21,433条账务待归因，含一条缺失精确dedup且权益关联不一致的记录 |
| 集成/发布 | 主线程/发布负责人 | #397已合并部署；下一批累计改动后再提一个PR并完整监督CI/部署，不为文档单独发布 |

具体功能域采用[功能矩阵](./feature-matrix.md)和[开发清单](./DEVELOPMENT_CHECKLIST.md)，不再把旧候选条目复制为新开发工作。保留Fork的066/070定时测试、outbox、273模型范围及已部署282/334/337/338；同名异内容迁移不可覆盖，不按上游数字前缀直接搬运。request-capture与Fork自有管理员请求台账不是同一功能，额外运行时/页面仍按范围合同评审。

## 历史团队任务（#393发布前快照，以下候选状态不代表当前）

> **2026-10-31 集成候选**：工作分支 `codex/v2177-cumulative-integration-20261029` 基于生产 `21dd9af2239c2c7f8259e41e31e21156131622b4`，当前改动未提交。候选包含 Priority 配置化分位数/半开窗口修复、Quality ownership 并发/删除保护、Jev choice judge、规则模板/预设、管理员质量规则 API/UI，以及凭据失败、Token Guard、OAuth reauth、内容审查用户管理和滚动 Usage 的累计改动；Go 全量测试、前端全量测试、lint、类型检查、生产构建、i18n、设计治理和 Fork integrity 已通过。候选尚未提交、推送、创建 PR 或部署；生产 #392 deployment `6ae303acea16512cf7419a17` 的 Zeabur check 成功，首页和 `/health` 为 200。

1. **集成范围/版本矩阵**：继续把共同祖先至 v2.17.7 的相关功能域逐项归为 Fork 等价、纳入迁移、明确排除或阻断，并附代码/测试/迁移证据；不搬运整树、不重做已部署功能。
2. **Priority**：本候选修复首发、重试和 WS 准入绕过；仍需完成完整上游 hard-stop、penalty clock、quality round 联动与跨协议回归。
3. **Quality/Jev**：本候选新增 `/admin/account-quality` 管理员入口、质量规则创建/编辑/运行/历史、ownership 预览和 reconcile/restore/delete 操作，并补充 Jev choice judge、规则模板/预设和同步 runner；仍缺 live regression、跨计划历史、分组/模型处罚、生产实时回归与真实恢复验证。
4. **Credential**：继续核清凭证失败 allowlist、安全展示、writer/reader/API/worker 和上游 OAuth reauth 的完整合同；候选测试通过不等于整条上游能力已迁完。
5. **迁移和生产预检**：隔离 PostgreSQL 验证候选迁移、完整文件名/checksum/顺序/锁耗时；部署前用 Zeabur 只读核实迁移账本和兼容影响。当前环境没有 Zeabur CLI，未做本轮生产数据库读取。
6. **统一交付门禁**：`check-fork-integrity.sh`、Go/前端全量测试、lint、类型检查、生产构建、i18n、设计治理和文档检查均已通过；视觉记录明确静态评审板不是生产浏览器验收截图。候选可进入逐文件复核和一次性提交，但生产管理员浏览器验收仍必须由用户本机完成，不能用静态图替代。
7. **部署后验收**：管理员继续验收 Quality、moderation 写动作、渠道运维和优先调度；Usage 正常及 #384 请求台账 admin-only 已确认，不重复排队，未来仅在相关权限代码变更时回归。每个生产批次仍核对实际部署 SHA、数据库迁移、日志和关键 API。
8. **独立风险跟进，不扩入本批功能开发**：对约 2 万条 `settlement_pending` 做脱敏只读业务/财务抽样；复核部署日志窗口、PostgreSQL crash recovery 原因及历史日卡审计语义。不得补扣、批量改状态或清理历史数据。
9. **明确排除/阻断**：request capture、support/harvest 等正文采集先完成隐私范围合同；security penalty 在全文访问、保留期限、自动 TTL 和审计约束明确前不迁入。

## 2026-10-31 历史状态（#393发布前）

- 生产：`play/main@21dd9af2239c2c7f8259e41e31e21156131622b4`；GitHub Zeabur check 成功，deployment `6ae303acea16512cf7419a17`；首页和 `/health` HTTP 200。候选未部署。
- 工作区：`codex/v2177-cumulative-integration-20261029` 基于生产 SHA，候选改动未提交。没有目标分支开放 PR。
- 本地验证：Go 全量测试、前端全量 Vitest、ESLint、Vue 类型检查、前端 production build、i18n 检查、设计治理、Fork integrity、文档链接和交接检查通过。候选仍未执行隔离 PostgreSQL 迁移预检，也没有远端 PR CI 或生产部署证据。
- 上游矩阵：446 个 merge SHA 均已唯一映射并初步分流（121 reviewed、325 triaged）；生产证据、代码等价和管理员验收仍未闭环；`fully_migrated=false`。
- 功能工作包：Quality 的模板/预设/Jev choice judge 已完成代码与本地测试；Priority attempt evidence/quality round、Quality live regression/真实恢复、Credential 上游逐字段等价复审以及所有生产验收仍有缺口，详见上方队列。
- 生产数据：10 月 29 日快照中的约 2 万条 `settlement_pending`、迁移/日志与历史 entitlement 事项均未在本轮重新读取；保留为独立只读核验项，不据旧快照推断新部署后的数据库状态。

以下保留 2026-10-29 及更早的历史状态和采样，不代表当前部署 SHA。

## 历史状态（截至 2026-10-29）

- 来源锁定：已完成。
- 上游完整差异分类：初步分流已完成；446 个 SHA 唯一对应，121 reviewed、325 triaged。仍需关闭 split、product-scope、evidence、隐私/架构阻断，并为纳入项补代码、测试和生产证据。
- 后端/API：本地候选 `f0bf08935` 已通过 Priority/Quality 定向门禁；Credential 的 Python worker 和前端门禁已通过，Go 被 stale Wire provider 装配阻断；生产尚未应用。Quality 的 Jev/live runner/真实恢复仍未完成；request capture、support/harvest 和 security penalty 仍未完成或被阻断。
- 前端/交互：滚动用量和内容审查用户管理已在本地候选合并并通过全量门禁，未推送/未部署；已发布的渠道运维和调度页面仍需管理员本机验证真实操作、保存刷新、审计与普通用户无入口/无权访问。
- 数据库/计费：生产 ledger 有 416 条；409 个当前迁移均已应用且 checksum 匹配，7 个额外历史记录也匹配 Git 中退役前原文件；0 个当前迁移未应用、0 个 checksum 不匹配。282 结构与 gateway 275–281 关键 schema/约束均核验通过；billing link/task 孤儿、settled+unverified link 和 stale inflight 均为 0。需单列高优先级核验：19,538 条 settlement_pending 请求中 19,537 条无精确 billing link，绝大多数 usage unknown；不能据此认定已扣款或再次扣费，需业务/财务只读抽样对照旧 usage、余额账本、订阅扣额和 dedup/archive 证据。另有历史日卡数据（4,136 replay、5,095 holds、21 entitlements），当前代码无对应读写引用；hold/replay 聚合一致性检查通过。唯一 `active` 但到期的日卡记录属于 `redeem_code`，其对应订阅也已 expired，无 payment order link。另有 4 条套餐权益记录状态 `active` 但已过期，均无有效订阅/替代权益；代码动态过期判定和准入 SQL 会拒绝它们。历史状态不自动修复；迁移锁耗时仍待办；283 不纳入本批。
- CI 与本地门禁：此前候选 Go、前端全量 Vitest、ESLint、typecheck、设计治理、Fork integrity、构建均通过；本轮 Priority 定向 requestledger/repository/service 测试和 Quality 定向 service/repository/handler 测试通过。Credential 定向 Go 测试因耗时未完成，尚未合入主候选；尚未创建 PR，远端 CI 未运行。
- PR、合并、生产部署：PR #389 已合并；merge/deploy SHA 为 `5c16bc5b5965a7b0d7c38aa35671639f045f2dea`；Zeabur deployment `6ae1a532ea16512cf741806a` 成功；首页/健康 200；本地候选 `f0bf08935` 尚未推送，尚未达到整版同步或生产业务验收完成。

PR #389 提交前的本地完整 `make test-backend`、前端 lint/typecheck/全量测试/生产构建、设计治理、Fork integrity 和交接检查均通过；该 PR 没有远端 CI checks。部署后验证了生产分支 SHA、首页/health、构建资源及新 admin API 的未认证拒绝；生产迁移账本和 282、gateway 相关结构已完成只读核验。当前生产 audit 的 Zeabur 日志只有滚动最近 100 行，不能代替完整部署窗；另发现 19,538 条历史 settlement_pending 记录待业务归因。日卡/套餐权益已聚合核对，未发现当前有效权益被过期记录放行；历史审计语义也待业务核验。本批已部署，但以上缺口和未分类上游功能阻止将 `fully_migrated` 改成 true。当前 Usage 候选门禁状态见团队任务记录，不等同于 PR #389 当时已通过的治理检查。

## 生产交接验收清单（截至#397）

### 管理员现场检查

使用用户本机浏览器和现有管理员账号，不在交接文档中记录账号或令牌。内容审查列表加载/刷新已由用户再次确认正常，不重复验收；下表写动作仍各自待办：

| 检查 | 操作与通过证据 | 状态 |
|---|---|---|
| 渠道运维入口/权限 | 管理员打开 `/admin/channels/operations`，页面可加载；普通用户侧栏没有该入口，直达路由及 API 被拒绝 | 等管理员现场确认 |
| 渠道运维读写 | 查询渠道目录和运维快照；对一项无生产影响的配置做经批准的测试变更，保存后刷新仍一致；冲突 revision 返回可理解错误 | 等管理员现场确认；不要用真实流量开关做未经批准的试验 |
| 审计 | 对上述测试变更确认审计记录含操作者、目标 ID、结果和版本，不含凭证、token、原始上游响应 | 等管理员现场确认 |
| 优先调度 | 管理员打开 `/admin/priority-scheduling`；确认读取/保存/刷新一致、状态明确；候选不得绕过硬准入；无历史/质量数据时显示 unknown/unavailable 而非伪造通过 | 等管理员现场确认 |
| 质量多模型轮次 | 管理员在 `/admin/account-quality` 用低成本规则设 `observe_only`，跑2模型×2次；保存刷新后确认同一轮4条结果、跨计划历史和Priority逐模型归因 | 等管理员现场确认；模型请求会产生少量上游用量 |
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

## 2026-10-28 最新生产复核覆盖

## 2026-10-29 历史生产复核（不覆盖#396）

### 2026-10-29 后续只读复核（覆盖本节较早统计）

- 当前本地候选为 `codex/v2.17.7-moderation-release-20261028@f0bf08935`；生产仍运行 `play/main@5c16bc5b5965a7b0d7c38aa35671639f045f2dea`，deployment `6ae1a532ea16512cf741806a`。候选未推送、未创建 PR、未合并、未部署；`fully_migrated=false`。
- 本次应用服务滚动 100 条日志为 INFO 99、WARN 1、ERROR/FATAL 0。唯一 WARN 是 `account_select_failed`（模型 `gpt-5.6-sol`、pool 2、excluded 1、runtime_blocked 1）。这表明当前请求未从该池选出账号；手头证据尚不足以归因两个排除原因，也不能据此断定候选代码有问题。需从账号池状态、排除策略和 runtime block 指标继续只读追查。服务 API 仅提供最近滚动日志，不能证明整个部署窗口无异常。
- 生产 PostgreSQL `schema_migrations=416`，最新为 `282_channel_operations.sql`；内容审查用户状态、命中、动作表及 security penalty 表仍不存在。`/api/v1/requests` 返回 404，未认证 `/api/v1/admin/requests` 返回 401；生产首页与 `/health` 为 200。
- 最新只读聚合为 20,088 条 `gateway_requests.settlement_state=settlement_pending`，全部 `usage_unknown`；另见 4 条 pending 和 1 条 known 用量记录。该状态不证明扣款结果，必须按只读 usage、余额流水、订阅扣额及幂等证据抽样归因；不得批量标记 settled、补扣或清理记录。
- Go `./...` 在当前候选通过。前端全量 Vitest 已通过（513 个文件、3,957 项通过、3 个文件/8 项跳过）；`pnpm` shim 缺失导致原组合脚本中断，随后直接运行本地 ESLint、设计治理、i18n 完整性、`vue-tsc --noEmit`、`vite build` 与 `scripts/check-fork-integrity.sh` 均通过。构建有现存 chunk 导入方式及 Browserslist 数据时效提示，不影响退出状态。完整本地门禁通过；GitHub 远端 PR CI 尚未运行。
- 本轮生产读取保持只读。部分早期只读 SQL 曾因字段名或引号错误失败，没有生产写入；这些是审计命令错误，不是应用故障。

以下两次 10 月 29 日采样记录较早的同日观察；日志计数和数据库聚合以本节开头的“后续只读复核”为准。

- Zeabur 项目 `6a51de14c2881a93656fa4c5`、环境 `6a51de14104975fcb46761c3` 已通过 CLI 读取；应用服务、PostgreSQL 和 Redis 当前为 `RUNNING`。应用 deployment 仍为 `6ae1a532ea16512cf741806a`，对应 `play/main@5c16bc5b5965a7b0d7c38aa35671639f045f2dea`；`/` 与 `/health` 均返回 200。
- 应用服务最近 100 条 runtime 日志为 INFO 84、WARN 16、ERROR/FATAL 0。WARN 已分类为 observe 模式无审核 API key，以及上游账号 403/503 引起的重试、failover 和 transient cooldown；当前没有证据把这些 WARN 归因于本地候选代码。Zeabur CLI 只提供服务级滚动窗口，不能证明完整部署窗口无异常。
- PostgreSQL 只读查询显示 `schema_migrations` 416 条，最新文件仍为 `282_channel_operations.sql`；`content_moderation_user_states`、`content_moderation_user_hits`、`content_moderation_user_actions` 均不存在。候选 moderation 路由和迁移尚未进入生产。
- `gateway_requests` 当前约 2 万条 `settlement_pending`，绝大多数 `usage_unknown` 且没有精确 billing link；该状态不等于已扣款或未扣款，业务/财务必须按只读 usage、余额流水、订阅扣额和幂等证据抽样归因。禁止批量标记 settled、补扣或清理历史记录。

以下为当时覆盖较早采样的历史结果，不覆盖首段#396或当前状态表：

本次状态修订：候选 moderation 用户管理与滚动 24 小时用量已在本地合并提交 `00b50ef83`，并通过本地全量门禁；当前生产仍未应用这些迁移和前端候选。

- Zeabur 当前服务 `sub2api-wortic` 与 PostgreSQL 均为 `RUNNING`；Git trigger 为 `refs/heads/play/main`，部署 SHA 为 `5c16bc5b5965a7b0d7c38aa35671639f045f2dea`，首页和 `/health` 返回 200。Deployment API 的 `status=RUNNING` 与 `finishedAt` 同时出现，不能单凭 status 字段宣称终态成功。
- 应用最新滚动 100 条为 INFO 84、WARN 16、ERROR/FATAL 0；WARN 主要是 `content_moderation.skip_no_audit_api_keys`，以及上游账号 403/503 导致的重试、切换和暂时熔断。当前采样没有应用 ERROR/FATAL，不把这些 WARN 直接归因于本地候选代码。
- PostgreSQL `schema_migrations` 为 416 条；`293/296/298/301` 均不存在。`content_moderation_logs` 存在，moderation user state/hits/actions 与 security penalty 表不存在。候选 moderation 路由在生产返回 404，证明候选尚未部署；应用启动迁移路径曾在已部署迁移写入后运行，缺失候选迁移是代码版本差异而非迁移器未运行证据。
- 最新只读聚合约有 2 万条 `settlement_pending`，绝大多数为 `usage_unknown` 且没有精确 billing link。该状态不证明已扣款，必须等待业务/财务只读归因，不得批量改状态或补扣。
- PostgreSQL 最近日志中的 ERROR/FATAL 来自本轮只读审计命令的列名/超时参数错误及管理员终止并行 worker，没有证据表明是应用迁移或业务写入故障；生产日志原文不进入仓库。
