# v2.17.7 上游功能差异与开发清单

核验日期：2026-10-31（Asia/Shanghai）

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
| 已核实生产部署 | `play/main@aea9be1e6508c5c319cc763c5fe058bc8d4b4aed` | PR #395；Zeabur deployment `6ae5dc8eea16512cf741cda0`，构建完成且 RUNNING；首页/health 200；源码仍为 Fork `0.2.14` |
| 当前候选分支 | `codex/api-key-multi-group-20261031` / PR #396 | 从已部署 #395 隔离，API Key 多组本地测试/lint/build及PostgreSQL实测已通过；新head CI、合并/部署待完成 |
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
| P0 | 渠道运维 | #573，channel operations | #389 已部署，生产 API/页面技术证据已有 | 完成管理员保存、刷新、CAS 冲突、审计和上游增量回归；保留 Fork 的 `282_channel_operations.sql` |
| P0 | 优先调度与 admission | #315/#422/#508/#520/#572 | #389 已部署基础合同；完整 penalty/admission 未证明 | 补 attempt identity、质量 round、HTTP/WS hard-stop、penalty clock、sticky/fallback 和观察数据回归 |
| P0 | 质量运维/Jev | #51/#174/#437/#447/#522/#528/#548/#550/#565/#575/#576 | #393 已部署 quality runtime、ownership、Jev、模板/预设、跨计划历史和恢复接口；320 迁移已应用 | 修复 AutoRestore 开关不生效和 unknown 被历史计为通过；多模型/parallel_count 执行、分组动作和模型隔离仍有真实缺口；现有恢复动作待管理员验收 |
| P0 | 凭证失败、token guard、reauth | #138/#178/#185/#192/#199/#224/#238/#440/#557/#560/#574 | 生产已有Fork token guard v2、reauth、refresh-failure reader/writer/API/UI；本批凭证定向回归通过，累计全量门禁见第5节 | 完成与上游逐字段行为复审、迁移账本核验和生产动作验收；不把 tag 外 Fork 功能冒充 v2.17.7 增量 |
| P0 | 内容审查用户管理 | #519/#558/#580；迁移 296/298/301 | #393 已部署；296/298/301 和状态/hits/actions 表及索引已核实；#394 列表修复已由管理员本机与生产 200 日志验证 | 继续验收 generation/reset、四类 exemption、owned unban、审计；独立启用已在审核记录行实现，不新增重复入口 |
| P0 | 精确滚动 24 小时用量 | #579 | #393 已部署 API/UI；用户确认 Usage 正常 | 保留已实现 `[now-24h, now)`；补分页/导出同窗、时区/DST 与日历清理实际验收，不重做运行时 |
| P0 | 迁移与账务安全 | 上游/Fork迁移按完整文件名和checksum处理；旧差异统计为分析基线快照 | 生产429 ledger，422当前SQL无缺失或checksum差异，另7历史记录；`settlement_pending`待归因 | 每个 SQL 按完整文件名、checksum、依赖和锁耗时预检；禁止 down、补扣或批量改结算状态 |
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

- [x] 已在从 #394 创建的隔离 worktree 固定分析基线、生产 SHA、上游 tag/commit 与 Fork VERSION；下一发布前重新确认实际 SHA。
- [x] 生成并核对 446 行结构化清单：[merge-inventory.tsv](./merge-inventory.tsv)，当前 13 列包括 `merge_sha`、`pr_number`、主题、变更路径、自动功能域、人工复核状态、处置、生产证据、负责人、依赖、测试和回滚字段；446 个 SHA 唯一且与来源范围一致。
- [x] 完成功能域和初步处置分流：`reviewed=121`、`triaged=325`、`unreviewed=0`。`triaged` 仅表示按 merge/路径证据归域并分派后续，不代表代码等价、测试通过或生产已部署。
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

- [ ] 渠道运维：确认管理员 route/API、revision CAS、成员归属、只读 upstream URL、审计字段和凭证脱敏。
- [ ] 优先调度：确认评分只影响排序，硬准入不可被评分绕过；历史 unknown 显示 unknown，不当作惩罚。
- [x] 已部署按 attempt_id 幂等的物理 attempt 存储与 30 天清理；model/endpoint/group/provider 路由字段及 HTTP 语义观测已有测试。WS 原生 latency/语义观测仍待单独补证，不能用 WS admission 测试替代。
- [x] stop-availability、sticky/fallback、HTTP/WS 最终 admission、retry send 重检、配置化分位数和惩罚时钟已有部署代码/回归；不重新实现。
- [ ] 生产两种 reader 在 #394 仍未填 QualitySamples/QualityPassed；累计候选已接入 account/model 完整 quality round、独立 QualitySince/cache 与337稳定配置revision。本地全量门禁已通过，真实 PostgreSQL CI 和生产验证仍待完成。
- [ ] 完成管理员真实浏览器保存/刷新/冲突回读；普通用户侧栏、直达路由和 API 均拒绝。

### P0-C：质量运维/Jev

- [x] 迁移和模型：已补质量模板/预设、quality round/judgment、ownership/version、隔离状态；Pelican/provider 扩展仍按范围阻断。
- [x] 运行器基础：已接入 scheduled runner、Jev choice 严格解析、低置信度 `unknown`、超时/取消/限定重试和幂等结果写入；模型发现/凭据替换等live regression具体缺口见下项。
- [x] 恢复基础：已实现 ownership reconcile/restore、冲突检测、孤儿状态保护、删除确认和审计；真实 restriction 恢复仍需生产验收。
- [x] API/UI：管理员列表、筛选、模板/预设切换、运行详情、历史、ownership 操作和空/错/禁用态已接入，中英文资源与组件测试通过。
- [x] 已有 Anthropic/OpenAI/Grok fake provider prompt 发送测试与 Jev HTTP 低置信度/重试/取消测试；#394 生产迁移逐文件校验通过，不将这些重新列为全缺。
- [x] 本累计批次实现 AutoRestore 显式开关、历史 unknown/incomplete 不标为通过；定向回归与一次统一发布门禁已完成，生产发布仍待完成。
- [ ] 后续完整包：ModelIDs/ParallelCount 实际执行、按模型限制/分组动作、live 模型发现与替换；保留隔离 PG ownership 全链和真实管理员恢复验收缺口。

### P0-D：凭证失败、token guard 和 reauth

- [x] #393既有Wire/provider一致性修复及当时Go全量通过证据保留；本批统一门禁见第5节；不得以旧生成文件掩盖缺依赖。
- [x] Fork 已部署 token guard v2 的 generation、并发刷新、超时、撤销、重登预算及 reauth worker；不从零重做。
- [x] 已修复历史 ProbeDetail 读取安全投影，并核对 structured error.type/code 对 invalid_grant refresh race 的影响；运行时清理期限、管理员最小读取/动作仍须逐项验收。
- [x] writer 已有 allowlist 安全分类，本批补齐历史 reader 安全投影与 structured refresh marker；禁止保存 token/Authorization/原始响应。
- [x] writer/reader、admin API/UI、重试与人工重登已有代码与测试，334 已在生产应用；历史详情安全读出正修复，管理员真实动作验收仍待完成。
- [x] 渠道/凭证等迁移已按Fork编号适配（282/334等），生产按完整文件名与checksum核对；不覆盖同号上游迁移。当前新337质量规则revision仍待预检/CI/发布后核验。
- [x] mock HTTP、并发刷新、worker 重启、网络超时、上游 401/403/429/5xx 和旧 token generation 定向测试已通过；仍需生产迁移和管理员动作验收。

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
- [ ] 在归因完成前禁止补扣、标记 settled、删除或重放历史请求；将未知归因保持为未知并记录负责人。
- [ ] GPT-6.1 Sol 只读抽查长上下文阈值、OAuth/Setup Token 豁免、API key 边界、模型映射、冻结价格和最终扣费。

### P1：按产品范围立项的整包功能

- [x] 只读差异核对：#562 token rotation 已由 Fork 的 session generation、跨标签页刷新协调和成对持久化等价实现，不重复开发；#504 是连接诊断，不是分组倍率。
- [ ] 本批 #479 API key 多组绑定/择组/计费快照已进入 PR #396，迁移338，测试/发布待完成。其余未纳入本批：#567 用户与 Key 的上传/下载字节计量、游标报表和异步汇总；#504 连接诊断查询与可选 edge collector；#570 model/group-scoped multiplier。均需产品范围、保留期限和计费兼容评审后立项。

- [ ] **BPS/Excel OAuth**：认证、能力发现、工具目录、图片/附件限制、proxy、rate-limit、history、worker、重登、错误分类、计费和前端开关。
- [ ] **Pelican**：账号/模型 eligibility、scheduled/group tests、结果 API、成本、展示、历史分页、CSP/nonce 和失败恢复。
- [ ] **Prism/Astra**：OAuth bridge、session cache、并发 turn、工具/图片、byte budget、lease、model scope、gateway history、cancellation 和 settlement。
- [ ] **Mihomo/Monitor**：静态代理绑定、管理 API、节点/代理池、V2/V3 监控、idle probe、卡片 timeline、软删除和超时。
- [ ] **Regional runtime**：多实例 gateway、local auth、scheduler sync、transport、slot lease、stream、regional ledger、admission、pricing 和部署验收。
- [ ] **Request/Support/Harvest**：request capture/session、流分类、support tickets、ticket admission、harvest flow、node learning、proxy borrow、观察者权限和保留期限。
- [ ] **Provider/Media**：OpenAI SIWC/2FA、Grok realtime/TTS/video、MiniMax video、Pelican/额外 image model、SSE/WS、上传和 usage attribution。
- [ ] **安全审计**：IP abuse guard、cyber evidence、fingerprint block、security workflow、mail exemption、类型安全 audit category；#492 prompt 全文暂阻断。
- [ ] **管理/移动/国际化**：API key 多组、group multiplier、用户流量、account batch/export、移动表格、菜单本地化和错误状态。

## 5. 统一验收和发布门禁

### 代码和测试

- [x] 当前累计候选后端完整测试、unit、静态检查与生产构建通过；各修复已有定向 RED→GREEN 和双审查。真实 PostgreSQL 集成测试仍由 PR CI 执行。
- [x] 当前累计候选前端Vitest：519文件/4,009项通过，3文件/8项跳过；ESLint、vue-tsc与设计治理通过，i18n已通过。
- [x] 当前累计候选前端生产构建通过（`vite build`）。
- [ ] migration checksum、真实 PostgreSQL 集成执行、API contract、权限矩阵、并发/CAS、mock upstream 和回滚测试：本地适用定向测试通过，真实 PostgreSQL 执行待 PR CI。
- [ ] 所有管理员页面检查 loading、empty、error、disabled、save/refresh、mobile/desktop、light/dark、zh/en；不得用假 API 或静态原型替代真实路由。

### 发布前

- [ ] 提交后运行 `python3 scripts/check_upstream_release.py --offline --strict`；当前候选仍有选择性未提交改动，严格检查须在单一累计提交后运行。
- [ ] 复核 446 行 merge 覆盖清单无 `unreviewed/unclassified`；所有纳入项另须具备实现、测试、依赖、owner、rollback 和生产证据。仅有 `triaged` 状态不能通过发布门禁。
- [ ] 数据库负责人完成只读 schema/lock/ledger 预检；安全负责人确认日志、错误、prompt、token 和凭证均不落原文。
- [ ] 只创建一个目标 `play/main` 的同步 PR；PR head、CI 状态、合并 SHA 和镜像 SHA 分开记录。

### 发布后

以下门禁对当前未发布累计候选继续待办；#394 的既有部署证据见批次账本，不重复列为未部署。

- [ ] 记录 Zeabur deployment ID、精确 Git SHA、容器/worker 版本、`/health`、首页和关键 admin API 响应。
- [ ] 核对 migration ledger、表/索引/约束/行数、启动锁耗时和应用完整部署窗口日志；滚动 100 行不能代替完整窗口。
- [ ] 管理员现场验收渠道运维、优先调度、Usage、moderation、质量和凭证页面；普通用户验证无越权入口和 API 访问。
- [ ] 对账本/账务只读抽样只记录脱敏证据；出现权限放宽、迁移不一致、5xx 或 SHA 不符立即停止批次并按预案回滚。
- [ ] 只有所有纳入项完成生产证据、管理员验收和回滚证明后，才把 `fully_migrated` 改为 `true`。

## 6. 完整范围复现命令

以下命令用于生成逐条审计输入。输出必须归档到同步批次，不得只保留摘要；`446` 是当前 tag 的预期行数。

```bash
cd /data/projects/sub2api-zeabur-go-toolchain
BASE=efe9aab1e4ec89a42ba45e8dac20e882c5409a6a  # merge coverage / common ancestor
TAG=refs/remotes/upstream-ranxi/releases/v2.17.7
PROD=3da3cb0e953fe751b50a2f6f446e8c24a62c80b2

git log --first-parent --merges --format='%H%x09%h%x09%s' "$BASE..$TAG" \
  > /tmp/sub2api-v2177-merge-inventory.tsv
test "$(wc -l < /tmp/sub2api-v2177-merge-inventory.tsv)" -eq 446

# Auditable inventory with changed paths and explicit pending decisions:
# docs/upstream-migrations/v2.17.7/merge-inventory.tsv

git diff --no-renames --name-status "$PROD" "$TAG" \
  > /tmp/sub2api-v2177-production-diff.tsv
python3 scripts/check_upstream_release.py --offline
```

每个 inventory 行保留：`path_domains_auto`、`review_status`、`disposition`、`production_evidence`、`owner`、`dependencies`、`tests` 和 `rollback`。当前 `reviewed=121`、`triaged=325`，#519/#558/#579/#580 已补限定生产证据、owner、测试和回退，其余仍有 `unverified` 或未映射项；分流完成不等于功能迁移完成。

## 7. 当前结论

- 已部署且应保留：Fork 的支付/权益/图片/增长/模型目录等产品能力、管理员专属请求台账，以及 #389 已上线的渠道运维、优先调度基础和 GPT-6.1 Sol 计费合同。
- #393/#394 已上线：精确滚动 24 小时 Usage、moderation 用户管理、质量 ownership/Jev/template/preset/跨计划历史与管理员页面、凭证失败安全分类。#394 列表刷新已由生产 5 次 HTTP 200 与用户本机反馈确认；这不替代其他写动作验收。
- 本批真实修复：质量 AutoRestore 开关与历史 unknown 判断、凭证历史详情安全读出及 invalid_grant 结构化错误差异。后续仍有质量多模型/并发/live/分组动作、Priority quality round/WS 观测、446 merge split/evidence/product-scope、账务只读归因与管理员真实验收。
- 明确阻断：上游 #492 security penalty 的明文 `full_prompt` 副本、跨用户管理员读取和无 TTL 合同；在最小读取、自动清理、保留期限、审计和回滚确定前不迁移。
- 未决范围：BPS/Excel、Pelican、Prism/Astra、Mihomo、区域运行时、request/support/harvest 和供应商媒体能力；每项必须先通过产品范围、隐私和基础设施评审，再进入 P1 开发包。
