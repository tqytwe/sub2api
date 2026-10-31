# v2.17.7 上游功能差异与开发清单

> **#398 发布后复核（2026-11-01 UTC+8）**：PR #398 已合并为 `play/main@db76a7e2a0a8c24818efad1fad6623fee28ab706`；Zeabur deployment `6ae65455ea16512cf741d4c8` SHA 一致，`finishedAt=2026-10-31T20:53:58.935Z`，状态 `RUNNING`。构建以 `build completed` 结束，无失败/failed/panic；应用、PostgreSQL、Redis 均 `RUNNING`，域名 `PROVISIONED`，首页与 `/health` 均 HTTP 200。运行日志筛查 ERROR/FATAL/PANIC 为 0，仅 1 条既有 `content_moderation.skip_no_audit_api_keys` WARN。未认证访问质量、渠道、内容审查用户、请求台账和 Usage 管理接口均返回 `401 application/json`。生产 PostgreSQL 只读事务已显式 `ROLLBACK`：`schema_migrations=431`、最新 `338_api_key_groups.sql`、无效索引 `0`，抽查 15 个关键迁移 checksum 全部匹配；#398 不含 SQL migration，本批无生产写入。质量管理员组合验收、Key/倍率验收、账务逐笔归因和其他业务写动作仍待完成，`fully_migrated=false`。

> **#396 发布后复核（2026-11-01 UTC+8）**：新 head `f09b7230fce899370db56f4e98350e15ed1ee8a6` 的9项CI全部通过，已普通merge至 `play/main@73f9ba38b4d61ad5a0e0ac05ab2004d4c9775444`，与受测head源码一致。Zeabur deployment `6ae61c6cea16512cf741d279` 的SHA一致，构建镜像导出完成、`finishedAt=2026-10-31T16:56:01.268Z`、RUNNING，主站/API health200。数据库只读复核431条ledger，424个当前SQL全部匹配、7条历史；338表/主键/2外键/索引存在，invalid indexes=0，事务ROLLBACK。最近100行应用日志ERROR/FATAL/PANIC为0，WARN为缺少审核API Key；CLI有限窗口不证明全启动日志无异常。Keys接口一次探测超时后主站/API重查均401，其余未认证接口401。技术部署核验通过，本批Key与管理员倍率的本机浏览器验收仍待完成；内容审查刷新已由用户确认，不重验。完整正文/harvest继续阻断，`fully_migrated=false`。详细证据和剩余验收见 [PR #396](https://github.com/tqytwe/sub2api/pull/396)。本段为发布后本地补记，随下一批累计提交，不为文档再次部署。

> **#397 发布后复核（2026-11-01 UTC+8）**：PR head `70524682fd0b27de9fb7fd2dbc474906a86a362f` 的9项CI全部通过，普通merge至 `play/main@f7193d639980840ae9fad66376355f4243d82a35`。Zeabur deployment `6ae634e9ea16512cf741d3ca` SHA一致、`finishedAt=2026-10-31T18:41:14.032Z`、状态RUNNING；主站/API health均200，部署启动日志74行无ERROR/WARN/FATAL/PANIC。只读事务核对431条migration ledger：本地424条全部存在且checksum匹配、7条历史记录；本批无SQL/schema migration；数据库现有索引有效。生产质量配置仍为1条启用的`observe_only`单模型单次规则；部署后尚无质量轮次样本，未产生正文。最近100条运行日志只有2条审核API Key未配置提示，无服务错误。生产内容审查用户管理刷新已由用户确认正常；#397多模型/重复轮次和历史结果的管理员本机验收仍待完成，`fully_migrated=false`。上一正常deployment `6ae61c6cea16512cf741d279` 保留为故障回滚目标。详细实现与门禁见 [PR #397](https://github.com/tqytwe/sub2api/pull/397)。本段为发布后本地补记，随下一批累计提交，不为文档再次部署。

> **#397 后续生产复核（2026-11-01 03:04 UTC+8）**：Zeabur CLI 已认证并能读取项目；生产仍为 `play/main@f7193d639980840ae9fad66376355f4243d82a35`。用户确认“内容审查用户管理”刷新正常、无 `internal error`。生产请求台账只读事务（显式 `BEGIN READ ONLY` / `ROLLBACK`）现有 `settlement_pending` 21,433 条，均早于 #397 部署；当前 `inflight` 为0。状态分布为 cancelled/usage_unknown 2,458、failed/usage_unknown 18,757、interrupted/usage_unknown 199、succeeded/known 1、succeeded/usage_unknown 18；失败标签计数为 upstream_error 13,108、internal_error 4,871、invalid_request 552、rate_limited 139、forbidden 87，标签不等于根因。仅1条有 billing link，verified/applied 均0；没有匹配指纹的实时/归档 dedup，也没有精确钱包流水。有1条精确 usage log，但 `billing_settled=false`；关联订阅与权益记录存在，usage log 的 subscription ID 匹配而 entitlement ID 不匹配；没有对应 subscription receipt/outbox。查询未输出账户或请求标识。该证据不足以证明已扣款、未扣款或已消耗订阅额度；逐笔订阅扣额、dedup/archive 与失败原因归因继续待办，禁止补扣、改状态、删除或重放。质量多模型/重复轮次管理员验收仍待用户本机操作，`fully_migrated=false`。

核验日期：2026-11-01（Asia/Shanghai）

这份清单回答三个问题：上游 `v2.17.7` 有什么、生产已经有什么、还缺什么以及怎样交付。它是开发和发布门禁，不是把上游目录直接覆盖到 Fork 的合并计划。

## 1. 核验基线

| 项目 | 已核实值 | 解释 |
|---|---|---|
| 上游仓库 | `ranxi2001/sub2api` | 直接上游，来源锁见 [`source-lock.json`](../source-lock.json) |
| 上游版本 | `v2.17.7` / `9620d315f72a7dbf04fa10fca02970e1e410805e` | tag object 为 `af9a09cf92c1cbfdcf793862122d46a5a29a6da1` |
| 来源验证 | 在线 `check_upstream_release.py` 通过 | release、tag object、commit 和精确版本解析均匹配来源锁 |
| 共同祖先 | `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a` | 上游共同祖先源码版本为 `0.2.5` |
| 本地差异分析基线 | `526f09d36fa4d0a69af823fec2fc5a9ed495193f` | `source-lock.json.analysis_base`；供 Fork 对目标 tag 的差异报告使用，不替代上游共同祖先 |
| 上游范围 | 2,155 commits、446 个 first-parent merge、3,809 个路径变化 | 这是审查范围，不是待搬运文件数 |
| 已核实生产部署 | `play/main@db76a7e2a0a8c24818efad1fad6623fee28ab706` | PR #398；Zeabur deployment `6ae65455ea16512cf741d4c8` RUNNING；应用/PostgreSQL/Redis RUNNING；主站/API health 200；源码仍为 Fork `0.2.14` |
| 当前批次验收 | PR #398 已合并并部署 | 质量历史查询、WebSocket优先调度归因和空 delta 语义修复已部署；继承#397多模型/重复probe、整轮事务/历史与Priority模型归因；管理员本机验收仍待完成，详细组合证据见第5节 |
| 当前整版状态 | `fully_migrated=false` | 不能用健康检查、单元测试或一次部署成功替代功能闭环 |

生产基线、上游 tag、候选分支和本地远端引用必须在每次发布前分别记录。若线上实际部署 SHA 已变化，先更新本表和证据，再重新计算下面所有状态。

## 2. 状态定义

- **已部署等价**：生产 SHA 中有实现和测试，且有 API/权限/数据或浏览器证据；仅有代码不算。
- **候选待发布**：隔离候选有实现和本地门禁，但生产未应用；不能对用户承诺可用。
- **缺失开发**：上游能力在产品范围内，生产没有可证明的等价实现。
- **范围待定**：需要产品、隐私、运营或架构决策后才能排期；不能静默跳过。
- **阻断**：存在迁移依赖、权限、数据、隐私或回滚风险；必须先解除阻断。
- **Fork 保留**：上游没有同等业务合同，生产 Fork 能力继续保留，不能被上游同步覆盖。

## 3. 发布决策总表

| 优先级 | 功能域 | 上游证据 | 生产状态 | 开发结论 |
|---|---|---|---|---|
| P0 | 来源、版本和 446 merge 覆盖 | v2.17.7 tag；共同祖先至 tag 的完整提交范围 | 446 行清单与 SHA 范围核对通过；整版证据未逐条关闭 | 继续按具体行为补证和拆分跨域提交；每行必须明确等价、适配、缺失、排除或阻断，不重复生成清单 |
| P0 | 渠道运维 | #573，channel operations | #389 已部署 CRUD/catalog/CAS/审计；质量统计固定返回 `unsupported_scope`，未读取质量轮次 | 当前候选已接入冻结配置/判题结果/计划轮次查询，复用已有 quality tables、无新增迁移；单测与隔离 PostgreSQL 集成通过，PR CI 和管理员页面验收后才能关闭 |
| P0 | 优先调度与 admission | #315/#422/#508/#520/#572 | #389/#395 已部署硬准入、attempt与单模型质量round；#397已部署多模型轮次及逐模型归因；本批接入原生WS首个解析确认语义事件的attempt观测，单测及隔离PostgreSQL回归通过 | 管理员验收保存/审计与hard-stop；下一批部署后核对真实WS观测行及模型/分组动作 |
| P0 | 质量运维/Jev | #51/#174/#437/#447/#522/#528/#548/#550/#565/#575/#576 | #393 runtime/ownership/Jev/模板/预设/历史，#395恢复/unknown修复，#397多模型整轮已部署 | 完成管理员多模型轮次及真实恢复动作验收；分组动作、模型隔离和live替换继续评估边界 |
| P0 | 凭证失败、token guard、reauth | #138/#178/#185/#192/#199/#224/#238/#440/#557/#560/#574 | 生产已有Fork token guard v2、reauth、refresh-failure reader/writer/API/UI；本批凭证定向回归通过，累计全量门禁见第5节 | 完成与上游逐字段行为复审、迁移账本核验和生产动作验收；不把 tag 外 Fork 功能冒充 v2.17.7 增量 |
| P0 | 内容审查用户管理 | #519/#558/#580；迁移 296/298/301 | #393 已部署；296/298/301 和状态/hits/actions 表及索引已核实；#394 列表修复通过生产验证；2026-11-01 用户再次确认刷新正常、无 internal error | 继续验收 generation/reset、四类 exemption、owned unban、审计；独立启用已在审核记录行实现，不新增重复入口 |
| P0 | 精确滚动 24 小时用量 | #579 | #393 已部署 API/UI；用户确认 Usage 正常 | 保留已实现 `[now-24h, now)`；补分页/导出同窗、时区/DST 与日历清理实际验收，不重做运行时 |
| P0 | 迁移与账务安全 | 上游/Fork迁移按完整文件名和checksum处理；旧差异统计为分析基线快照 | 生产431 ledger，424当前SQL无缺失或checksum差异，另7历史记录；21,433条 `settlement_pending` 的脱敏聚合及精确 usage/wallet 匹配已复核，逐笔归因未完成 | 每个 SQL 按完整文件名、checksum、依赖和锁耗时预检；禁止 down、补扣或批量改结算状态 |
| P1 | BPS/Excel OAuth 与附件协议 | #58/#62/#64/#68/#73/#74/#78/#79/#83/#86/#87/#89/#91/#96/#97/#98/#99/#102/#104/#105/#107/#110/#111/#118/#121/#122/#124/#125/#128/#131/#133/#134/#138/#142/#149/#151/#154/#156/#161/#173/#178/#183/#184/#185/#192/#197/#204/#207/#210/#217/#219/#220/#221/#224/#229/#231/#232/#242/#249/#358/#360/#361/#362/#363/#364/#368/#371/#373/#374/#375/#379/#460/#461/#473/#486/#506/#541 | 生产仅有 Fork 自有协议/图片/计费合同，未证明等价于上游 BPS runtime | 先定支持 provider 和流量范围；再按认证、能力发现、附件、重试、额度、计费和前端权限整体交付 |
| P1 | Pelican 质量、展示和测试 | #104/#166/#168/#174/#176/#188/#236/#253/#262/#263/#268/#271/#356/#378/#391/#392/#394/#396/#399/#400/#402/#404/#407/#409/#448/#449/#542 | Fork 有独立模型目录/质量基础；未证明完整 Pelican 闭环 | 先完成质量合同，再决定展示页、测试成本、历史分页和 group tests；禁止只搬页面 |
| P1 | Prism / Astra / 多轮工具 | #250/#251/#258/#260/#267/#269/#277/#278/#281/#284/#286/#287/#289/#292/#293/#294/#299/#300/#301/#304/#305/#306/#307/#310/#313/#314/#354/#399/#423/#424/#425/#426/#427/#428 | 生产没有统一上游 Prism/Astra 运行时证据 | 明确是否部署多实例/守护进程；完成 session、并发、工具、图片、预算、lease、缓存、计费和故障恢复合同 |
| P1 | Mihomo、代理池和渠道监控 | #115/#126/#128/#143/#157/#189/#200/#218/#254/#264/#273/#274/#275/#276/#282/#303/#309/#385/#404/#409 | Fork 有自有监控/代理/渠道实现，语义不等价 | 逐路由核对代理绑定、静态节点、健康探针、idle probe、卡片时间线、软删除和数据保留；同名迁移不得覆盖生产 |
| P1 | 区域运行时与网关 | #233/#241/#419/#420/#421/#422/#423/#424/#425/#426/#427/#428/#430 | 生产为单 Fork 部署合同；未证明 regional scheduler/ledger/admission 可运行 | 若纳入，先做多实例、跨区域认证、slot lease、同步、结算和回滚演练；否则写明确排除理由 |
| P1 | Request capture、support、harvest、observer | #15/#37/#77/#82/#113/#116/#117/#120/#132/#142/#159/#160/#334/#342/#385/#388/#391/#393/#398/#400/#402 | 未发现完整生产等价链；涉及原文、票据和代理能力 | 先签订隐私/保留/权限合同，再确定纳入；不能用静态页面替代 session、采集或工单闭环 |
| P1 | 用量、认证和模型权限 | #35/#36/#38/#85/#116/#185/#207/#214/#265/#287/#291/#302/#304/#307/#310/#413/#432/#458/#479/#490/#504/#510/#562/#567/#570/#577/#579 | GPT-6.1 Sol 计费和 admin-only 台账是 Fork 等价/已部署；其余为混合状态 | 逐行为核对 API key 多组、group multiplier、response model、连接诊断、token rotation、user traffic 和 rolling window |
| P1 | 协议与供应商媒体 | #31/#95/#155/#217/#242/#271/#289/#308/#338/#431/#453/#457/#459/#460/#466/#468/#473/#485/#490/#506/#542 | Fork 有独立 provider/catalog/image/video 功能；未证明上游每个协议等价 | 按真实支持矩阵验收认证、转发、SSE/WS、图片/视频、错误恢复、价格和 usage attribution |
| P1 | 安全、IP 风险和审计 | #32/#126/#161/#178/#183/#191/#205/#222/#284/#293/#308/#445/#487/#492/#513/#518/#523/#525/#529/#556/#563 | Fork 有自有 IP risk、security audit、prompt audit；留存和权限合同不同 | 保留 Fork 安全边界；#492 明文 `full_prompt`、跨用户管理员读取、无 TTL，暂不原样迁移 |
| P2 | 管理端、移动端、国际化和体验 | #126/#163/#177/#200/#294/#317/#346/#347/#351/#352/#354/#356/#404/#409/#443/#464/#568/#563 | 部分功能由 Fork 自有产品覆盖 | 只有对应后端合同、权限和构建资源齐备时接入；补中英文、移动视口、空/错/禁用态 |
| P2 | 构建、发布和文档 | #1/#11/#38/#147/#159/#160/#438/#450/#451/#517/#526/#553/#581 | Fork 有自己的 Zeabur/Go toolchain 和版本标识 | 补 strict source check、远端 CI、镜像/worker 矩阵、精确 SHA、回滚和文档链接检查 |

表中列出的 PR 是功能域索引；完整性门禁仍以第 6 节的 446 行逐条清单为准。一个 PR 可能包含修复、迁移和测试，不能只按标题判断“已完成”。

## 4. 可直接进入开发的工作包

### P0-A：范围锁定和逐 merge 覆盖

- [x] 本工作区源自 #396 并承载 #397 批次；下一功能批次须从当前已部署 `origin/play/main@f7193d639980840ae9fad66376355f4243d82a35` 新建独立分支/worktree，重新确认生产 SHA。
- [x] 生成并核对 446 行结构化清单：[merge-inventory.tsv](./merge-inventory.tsv)，当前 13 列包括 `merge_sha`、`pr_number`、主题、变更路径、自动功能域、人工复核状态、处置、生产证据、负责人、依赖、测试和回滚字段；446 个 SHA 唯一且与来源范围一致。
- [x] 完成功能域和初步处置分流：`reviewed=122`、`triaged=324`、`unreviewed=0`。`triaged` 仅表示按 merge/路径证据归域并分派后续，不代表代码等价、测试通过或生产已部署。
- [ ] 关闭 `needs_product_scope`、`needs_evidence`、`needs_split`、`scope_unresolved`、隐私和架构阻断项；目前已对 #519/#558/#579/#580 补上生产部署与限定验收证据，其余仍有 `unverified`，不得据局部证据计整版迁移完成。
- [ ] 所有 `out-of-scope` 必须写产品理由；所有 `equivalent` 必须链接生产代码和测试；所有 `missing` 必须进入本文件任务表。
- [ ] 清单生成后把 `fully_migrated` 继续保持 `false`，直到所有纳入项和发布验收关闭。

下表为 #393 前分流数量快照；当前精确处置以 TSV 为准，本批按生产证据更新已覆盖条目，不以这些旧计数推断剩余开发量：

| 处置 | 行数 | 下一步 |
|---|---:|---|
| `needs_product_scope` | 136 | 按现有 Fork 产品边界逐项决定纳入、排除或暂缓；不因上游存在就自动扩展产品 |
| `needs_evidence` | 67 | 对照生产代码、迁移账本、实际 API/UI 和测试补证 |
| `needs_split` | 35 | 按具体 hunk/路径拆分跨域提交，保留可独立纳入的修复 |
| `adaptation_required` | 74 | 按 Fork 路由、权限、数据和计费合同实现适配 |
| `partial_equivalent` | 34 | 补缺失行为和证据，不能计作完整等价 |
| `deferred_this_batch` | 26 | 本批明确暂缓；正文采集与 harvest 保持阻断 |
| `architecture_scope_blocked` / `scope_privacy_blocked` / `scope_unresolved` | 35 | 先解决架构、隐私或范围条件，再决定开发；不得静默带入 |

其余行包含已知 Fork 等价/保留、已部署待验收、版本策略排除等处置；具体 SHA、PR、路径和依赖见 TSV。上述分类全部是审计分流结论，不是生产验收结论。

### P0-B：渠道运维和优先调度

- [x] 源码差异已核实：管理员 route/API、revision CAS、成员归属、只读 upstream URL、审计字段和凭证脱敏已有；质量范围此前固定返回 `unsupported_scope`。上游读取冻结 `scheduled_test_results` 按 plan/round/model 判定完整度，生产质量面板因此缺少实测轮次。
- [x] 当前候选补齐质量历史查询：使用冻结模型/并发数和判题结果分类完整、降级、无法判断轮次；过滤按当前渠道成员/分组归属，不声称历史请求归因；游标分页只影响行，不缩小 summary 范围。管理员 UI/API 契约已存在，无新增迁移或前端入口。
- [x] 存储层和管理员 HTTP 透传单测、隔离 PostgreSQL 18.1 集成通过；覆盖双模型完整/降级/部分轮次、模型过滤、渠道/分组/账号范围和历史游标。PR CI 与管理员页面验收仍待完成。
- [x] 优先调度实现：原生WS仅对重复帧过滤后的解析确认语义事件记录当前attempt首观测；成功 completed/done 才持久化，失败、取消和中断不写成功观测。单测及隔离PostgreSQL回归通过。
- [ ] 优先调度生产验收：确认评分只影响排序，硬准入不可被评分绕过；历史 unknown 显示 unknown，不当作惩罚；部署后核对真实WS观测行。
- [x] 已部署按 attempt_id 幂等的物理 attempt 存储与 30 天清理；model/endpoint/group/provider 路由字段及 HTTP 语义观测已有测试。WS 原生 latency/语义观测仍待单独补证，不能用 WS admission 测试替代。
- [x] stop-availability、sticky/fallback、HTTP/WS 最终 admission、retry send 重检、配置化分位数和惩罚时钟已有部署代码/回归；不重新实现。
- [x] #395 已部署两种 reader 的单模型完整 quality round、独立 QualitySince/cache 和337稳定配置revision；适用 CI/数据库已通过。多模型实际快照/完整性本批继续适配，不重做已部署基础。
- [ ] 完成管理员真实浏览器保存/刷新/冲突回读；普通用户侧栏、直达路由和 API 均拒绝。

### P0-C：质量运维/Jev

- [x] 迁移和模型：已补质量模板/预设、quality round/judgment、ownership/version、隔离状态；Pelican/provider 扩展仍按范围阻断。
- [x] 运行器基础：已接入 scheduled runner、Jev choice 严格解析、低置信度 `unknown`、超时/取消/限定重试和幂等结果写入；模型发现/凭据替换等live regression具体缺口见下项。
- [x] 恢复基础：已实现 ownership reconcile/restore、冲突检测、孤儿状态保护、删除确认和审计；真实 restriction 恢复仍需生产验收。
- [x] API/UI：管理员列表、筛选、模板/预设切换、运行详情、历史、ownership 操作和空/错/禁用态已接入，中英文资源与组件测试通过。
- [x] 已有 Anthropic/OpenAI/Grok fake provider prompt 发送测试与 Jev HTTP 低置信度/重试/取消测试；#394 生产迁移逐文件校验通过，不将这些重新列为全缺。
- [x] #395 已部署 AutoRestore 显式开关、历史 unknown/incomplete 不标为通过；定向回归、统一发布门禁及 #395 精确SHA部署核验已完成；真实管理员动作仍需验收。
- [x] #397 已部署：ModelIDs/ParallelCount实际执行、整轮原子保存/保留与历史完整性、Priority逐模型归因；定向/全量测试、9项CI、构建及生产运行核验已通过，普通连通性规则保持原模型语义。
- [ ] #397 管理员本机验收：创建/编辑后保存刷新；显式选择测试规则和`observe_only`，执行2模型×2次并确认同一轮4条结果及跨计划历史；对照原单模型规则。后续为按模型限制/分组动作、live模型发现替换及真实管理员恢复验收。

### P0-D：凭证失败、token guard 和 reauth

- [x] #393既有Wire/provider一致性修复及当时Go全量通过证据保留；本批统一门禁见第5节；不得以旧生成文件掩盖缺依赖。
- [x] Fork 已部署 token guard v2 的 generation、并发刷新、超时、撤销、重登预算及 reauth worker；不从零重做。
- [x] 已修复历史 ProbeDetail 读取安全投影，并核对 structured error.type/code 对 invalid_grant refresh race 的影响；运行时清理期限、管理员最小读取/动作仍须逐项验收。
- [x] writer 已有 allowlist 安全分类，#395 已部署历史 reader 安全投影与 structured refresh marker；禁止保存 token/Authorization/原始响应。
- [x] writer/reader、admin API/UI、重试与人工重登已有代码与测试，334 已在生产应用；历史详情安全读出已随#395修复，管理员真实动作验收仍待完成。
- [x] 渠道/凭证等迁移已按Fork编号适配（282/334等），生产按完整文件名与checksum核对；不覆盖同号上游迁移。337质量规则revision已随#395部署；#397发布后只读复核431条ledger，424个当前SQL均匹配、7条历史；本批没有SQL/schema变更。
- [x] mock HTTP、并发刷新、worker 重启、网络超时、上游 401/403/429/5xx 和旧 token generation 定向测试已通过；适用生产迁移已核验，管理员动作仍待验收。

### P0-E：内容审查用户管理和滚动 Usage

- [x] 296/298/301 已部署，当前生产 ledger/checksum、三张目标表和两项索引只读核验通过；CAS 和权限回归已有，不再列待部署。
- [x] moderation 已支持按 UID 查找、状态 generation/count reset、四类 exemption、原因确认、owned unban、审计回读；不能放宽白名单或安全策略。
- [x] Usage 默认使用 `[now-24h, now)`，前后端传递同一 ISO 窗口；筛选、排序、分页、导出和错误请求沿用同一快照。
- [x] 日期范围清理只接受明确日历范围，UTC/东八区/DST/空数据已有测试；生产权限和数据验收仍待完成。
- [x] #394 的 migration ledger/表/索引、未认证 API 401、管理员列表刷新及修复 SQL 7 类筛选只读复核通过。
- [ ] 管理员真实写动作、普通用户 403、回滚前后数据一致性仍须按受影响范围逐项验证。

### P0-F：数据库、计费和历史数据

- [ ] 逐文件核对上游新增迁移、Fork 自有迁移、同名异内容和编号冲突；已部署迁移不可改写。
- [ ] 在隔离 PostgreSQL 测试升级、锁等待、重复启动、旧数据、失败回滚和 checksum；生产只读确认最高 migration、锁和耗时。
- [ ] 对历史 `settlement_pending` 做脱敏只读抽样：usage、余额流水、订阅扣额、billing link、dedup/archive、失败/取消原因逐项归因。
- [x] 2026-11-01 生产脱敏聚合：21,433条待结算记录、0条inflight、1条billing link、0 verified/applied、0精确已结算usage、0精确钱包流水；查询显式ROLLBACK且未输出标识。该聚合不构成逐笔归因或扣款结论。
- [ ] 在归因完成前禁止补扣、标记 settled、删除或重放历史请求；将未知归因保持为未知并记录负责人。
- [ ] GPT-6.1 Sol 只读抽查长上下文阈值、OAuth/Setup Token 豁免、API key 边界、模型映射、冻结价格和最终扣费。

### P1：按产品范围立项的整包功能

- [x] 只读差异核对：#562 token rotation 已由 Fork 的 session generation、跨标签页刷新协调和成对持久化等价实现，不重复开发；#504 是连接诊断，不是分组倍率。
- [x] #479 API key 多组绑定/择组/计费快照已随 PR #396 发布；迁移338已核验，本地/9项CI通过，Key和倍率本机验收待完成。其余未纳入本批：#567 用户与 Key 的上传/下载字节计量、游标报表和异步汇总；#504 连接诊断查询与可选 edge collector；#570 model/group-scoped multiplier。均需产品范围、保留期限和计费兼容评审后立项。

- [ ] **BPS/Excel OAuth**：认证、能力发现、工具目录、图片/附件限制、proxy、rate-limit、history、worker、重登、错误分类、计费和前端开关。
- [ ] **Pelican**：账号/模型 eligibility、scheduled/group tests、结果 API、成本、展示、历史分页、CSP/nonce 和失败恢复。
- [ ] **Prism/Astra**：OAuth bridge、session cache、并发 turn、工具/图片、byte budget、lease、model scope、gateway history、cancellation 和 settlement。
- [ ] **Mihomo/Monitor**：静态代理绑定、管理 API、节点/代理池、V2/V3 监控、idle probe、卡片 timeline、软删除和超时。
- [ ] **Regional runtime**：多实例 gateway、local auth、scheduler sync、transport、slot lease、stream、regional ledger、admission、pricing 和部署验收。
- [ ] **Request/Support/Harvest**：request capture/session、流分类、support tickets、ticket admission、harvest flow、node learning、proxy borrow、观察者权限和保留期限。
- [ ] **Provider/Media**：OpenAI SIWC/2FA、Grok realtime/TTS/video、MiniMax video、Pelican/额外 image model、SSE/WS、上传和 usage attribution。
- [ ] **安全审计**：IP abuse guard、cyber evidence、fingerprint block、security workflow、mail exemption、类型安全 audit category；#492 prompt 全文暂阻断。
- [ ] **管理/移动/国际化**：API key多组已随#396部署，不重复开发；group multiplier、用户流量、account batch/export、移动表格、菜单本地化和错误状态仍按范围逐项处理。

## 5. 统一验收和发布门禁

### 代码和测试

- [x] 定向RED→GREEN：多模型/整轮service与真实PostgreSQL集成通过；最后复审另发现目标处理误作用普通连通性规则，失败回归确认后，将validator置于Quality nil返回之后，且`prepareQualityPlan`对无Quality规则保留原primary。最终service定向退出0：`/data/tmp/quality-multiprobe-service-final2.log`。
- [x] `make test`退出0：其后端首轮在上述最后guard修正之前；前端已是最终冻结源码，519文件/4,026项通过，3文件/8项既有skip。最终ESLint、typecheck、设计检查通过；组件定向21项/i18n3项及11张修改后组件截图已有记录。旧批次4,009项不作为本批证据。
- [x] 最后guard修正后，完整`make test-backend`（普通测试及golangci-lint 0 issues）和完整`make test-backend-unit`均退出0，分别见`/data/tmp/quality-multiprobe-backend-final.log`、`/data/tmp/quality-multiprobe-backend-unit-final.log`；不把首轮组合命令描述为最终后端源码重跑。
- [x] `make build`退出0；最后后端修正后又完成embed构建，退出0，见`/data/tmp/quality-multiprobe-embed-build-final.log`。真实PG沿用行为相关仓库源码的既有实测；最后机械sort由最终普通测试覆盖，PR CI仍须运行。
- [x] 冻结源码后的最终Fork integrity退出0，`/data/tmp/quality-multiprobe-fork-final.log`末尾为`Fork integrity passed`；本轮四份文档12个本地链接、3个第5节锚点和diff复核通过。首轮Fork不代替该最终结果。
- [ ] 本批API contract、权限矩阵、并发/CAS、mock upstream与回滚覆盖完成最终汇总；已有真实PG定向结果不代替完整PR CI。
- [ ] 所有管理员页面检查 loading、empty、error、disabled、save/refresh、mobile/desktop、light/dark、zh/en；不得用假 API 或静态原型替代真实路由。

### 发布前

- [x] #397提交后运行 `python3 scripts/check_upstream_release.py --offline --strict`，退出0；最终受测head与合并head分别记录，不混为一谈。
- [ ] 复核 446 行 merge 覆盖清单无 `unreviewed/unclassified`；所有纳入项另须具备实现、测试、依赖、owner、rollback 和生产证据。仅有 `triaged` 状态不能通过发布门禁。
- [x] 本批生产规则只读预检：规则1条、enabled1、observe_only1，多模型0、重复probe0、max_samples1、超过候选上限0，显式ROLLBACK；未激活大批量检测/隔离，不作为真实多模型验收。
- [x] #397数据库只读核验确认无schema变更、migration ledger无缺失/checksum差异、invalid indexes=0，所有事务显式ROLLBACK；发布日志未见prompt、token或凭证正文，另有2条既有审核API Key未配置提示。
- [x] #397以一个目标 `play/main` 的PR合并发布；PR head、CI状态、合并SHA和Zeabur部署SHA分开记录。

### 发布后

以下门禁按批次更新；#397已完成的技术部署证据见本节顶部，未完成的管理员现场验收继续保留。

- [x] #398记录Zeabur deployment `6ae65455ea16512cf741d4c8`、Git SHA `db76a7e2a0a8c24818efad1fad6623fee28ab706`、RUNNING与两域名`/health` 200；应用/PostgreSQL/Redis均RUNNING，运行日志ERROR/FATAL/PANIC为0，仅1条既有审核API Key提示。
- [x] #397只读核对migration ledger、相关现有表/索引及有效性；本批无SQL/schema migration，事务显式ROLLBACK。
- [ ] 对本批受影响的质量配置/运行/历史及Priority质量信号做管理员现场验收，并回归普通用户无越权入口和API访问。既有内容审查列表刷新已验收不重做；其他批次未完成的写动作继续保留在账本。
- [ ] 对账本/账务只读抽样只记录脱敏证据；出现权限放宽、迁移不一致、5xx 或 SHA 不符立即停止批次并按预案回滚。
- [ ] 只有所有纳入项完成生产证据、管理员验收和回滚证明后，才把 `fully_migrated` 改为 `true`。

## 6. 完整范围复现命令

以下命令用于生成逐条审计输入。输出必须归档到同步批次，不得只保留摘要；`446` 是当前 tag 的预期行数。

```bash
cd /data/projects/sub2api-quality-multiprobe-20261101
BASE=efe9aab1e4ec89a42ba45e8dac20e882c5409a6a  # merge coverage / common ancestor
TAG=refs/remotes/upstream-ranxi/releases/v2.17.7
PROD=73f9ba38b4d61ad5a0e0ac05ab2004d4c9775444

git log --first-parent --merges --format='%H%x09%h%x09%s' "$BASE..$TAG" \
  > /tmp/sub2api-v2177-merge-inventory.tsv
test "$(wc -l < /tmp/sub2api-v2177-merge-inventory.tsv)" -eq 446

# Auditable inventory with changed paths and explicit pending decisions:
# docs/upstream-migrations/v2.17.7/merge-inventory.tsv

git diff --no-renames --name-status "$PROD" "$TAG" \
  > /tmp/sub2api-v2177-production-diff.tsv
python3 scripts/check_upstream_release.py --offline
```

每个 inventory 行保留：`path_domains_auto`、`review_status`、`disposition`、`production_evidence`、`owner`、`dependencies`、`tests` 和 `rollback`。当前 `reviewed=122`、`triaged=324`，#519/#558/#579/#580 已补限定生产证据、owner、测试和回退，其余仍有 `unverified` 或未映射项；分流完成不等于功能迁移完成。

## 7. 当前结论

- 已部署且应保留：Fork 的支付/权益/图片/增长/模型目录等产品能力、管理员专属请求台账，以及 #389 已上线的渠道运维、优先调度基础和 GPT-6.1 Sol 计费合同。
- #393/#394 已上线：精确滚动 24 小时 Usage、moderation 用户管理、质量 ownership/Jev/template/preset/跨计划历史与管理员页面、凭证失败安全分类。moderation 列表刷新已由生产 HTTP 200 与用户本机反馈确认；2026-11-01 用户确认刷新正常、无 internal error；这不替代其他写动作验收。
- #395 已部署 AutoRestore/unknown历史修复、凭证历史安全详情/invalid_grant和单模型Priority quality round；#396已部署API Key多组；#397已部署质量多模型/重复probe与整轮数据库/历史/模型归因。本批已补原生WS首个解析确认语义事件的attempt观测；live/分组动作、446 merge分流补证、历史账务归因和管理员真实验收继续待办。
- 明确阻断：上游 #492 security penalty 的明文 `full_prompt` 副本、跨用户管理员读取和无 TTL 合同；在最小读取、自动清理、保留期限、审计和回滚确定前不迁移。
- 未决范围：BPS/Excel、Pelican、Prism/Astra、Mihomo、区域运行时、request/support/harvest 和供应商媒体能力；每项必须先通过产品范围、隐私和基础设施评审，再进入 P1 开发包。
