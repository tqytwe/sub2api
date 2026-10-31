# v2.17.7 功能域矩阵

核验基线：生产 `play/main@db76a7e2a0a8c24818efad1fad6623fee28ab706`（PR #398），Zeabur deployment `6ae65455ea16512cf741d4c8`；应用、PostgreSQL、Redis 为 `RUNNING`，首页和 `/health` HTTP 200，运行日志 ERROR/FATAL/PANIC 为 0；生产只读账本为 431 条、最新 338、无效索引 0，关键迁移 checksum 抽查一致；管理员组合验收仍待完成；上游固定 `ranxi2001/sub2api@v2.17.7`。

2026-11-01通过Zeabur CLI核对#398精确SHA、RUNNING、应用/PostgreSQL/Redis和两域名health200。运行日志 ERROR/FATAL/PANIC 为0，只有1条既有缺少审核API Key提示。#397的生产只读核验确认431条ledger，424个当前SQL无缺失/checksum差异，7条历史记录；#398不含SQL/schema迁移，未写生产数据库。#395已部署质量恢复/unknown修复、单模型Priority quality round及凭证修复；#397已部署多模型/重复probe和整轮保存；#398补质量历史查询、WebSocket优先调度归因与空delta语义修复。#396 Key/倍率与#397/#398质量组合验收仍待管理员完成。内容审查列表刷新已由用户确认正常；该页面验收不代表审核动作/审核API已启用。

| 功能域 | 当前生产事实 | 本批与后续真实缺口 | 验收边界 |
|---|---|---|---|
| 来源/446 范围 | tag/commit 已锁定，446 merge SHA 唯一且覆盖准确；保留 Fork `0.2.14` 标识 | 持续关闭 split/evidence/product-scope，不重新生成相同清单 | `fully_migrated=false`；有代码不等于整版完成 |
| 渠道运维 | #389 已上线且 #393 保留；管理员路由、API、revision CAS、审计已有；生产质量统计原为固定 `unsupported_scope` | 当前候选查询冻结质量结果并按 plan/round/model 校验完整性，复用既有表；存储/API单测及隔离 PostgreSQL 18.1 集成通过 | 不新增页面/入口/迁移；管理员保存/刷新/冲突/审计及新质量数据现场验收仍待完成，查询按当前成员范围而非历史请求归因 |
| 优先调度 | HTTP/WS 最终 hard-stop、sticky/fallback/retry 重检、分位数与惩罚时钟、attempt 幂等/30天清理已部署 | 两种reader单模型quality round、稳定版本和有效期已随#395部署；本批补多模型完整round与逐模型归因，并接入原生WS首个解析确认语义事件的attempt观测；单测及隔离PostgreSQL回归通过 | 不将 admission 测试视为 latency observation 测试；unknown 不伪造为通过/处罚；生产需在下一批部署后核对观测行 |
| GPT-6.1 Sol 计费 | Fork 已有长上下文阈值、OAuth/Setup Token 豁免及测试 | 保留只读价格/用量/账务抽样项 | 无真实抽样不计整包完成，不补扣 |
| 滚动 Usage | #393 已部署，用户反馈正常；窗口/分页/导出快照与日历清理已有测试 | 本机精确窗口/导出/日期清理验收待补，不再开发另一套 | #579 部分生产验收通过 |
| 内容审查 | #393 已部署状态/hits/actions 与296/298/301；#394修好列表，用户本机确认，5次生产请求200 | generation/reset/四类exemption/owned unban写动作与审计待验收 | 独立启用已在审核记录行，get→update(status+reason)→get，不重置moderation状态；42项相关回归重跑通过，不复制上游第二套入口 |
| 管理员请求台账 | #384 已部署并经用户确认，普通用户入口已移除 | 只在受影响发布做权限回归，不重复列旧生产用户验收 | 保留 admin-only 与服务端拒绝 |
| 质量运维/Jev | #393 runner、ownership/fence、历史、模板/预设、Jev；#395恢复/unknown修复；#397多模型/多次probe、整轮事务/历史和模型信号已部署 | 管理员组合验收；分组/模型动作、随机题/普通LLM judge及live替换另列后续 | 真实管理员恢复/模板/预设动作待验收；现有fake provider/Jev tests不代表全部live合同 |
| 凭证失败/Token Guard/reauth | Fork guard v2/worker/refresh writer-reader/API/UI/预算已有，334已应用 | 历史安全详情及structured invalid_grant已随#395修复；network timeout分类和管理员动作继续对照 | 不新增同名迁移/平行worker，不读取真实凭据原文；机器测试不代替管理员动作 |
| 迁移与生产数据 | 424个当前SQL无缺失/漂移，431 ledger；#397无SQL/schema变更 | 21,433条待结算；1条link缺精确dedup/归档、usage未settled且权益关联不一致，逐笔归因继续 | 生产只读，禁止down/补扣/批量改状态 |
| 请求采集/support/harvest | 未纳入本批；用户明确保留阻断 | 最小读取、摘要/哈希、按需限期加密原文与自动TTL合同先行 | 不为版本同步引入正文采集 |
| 安全prompt审计 | Fork既有prompt审计与上游明文full_prompt合同不同 | #492全文/跨用户读取/无TTL保持阻断 | 不修改/清理生产历史原文 |
| BPS/Excel、Pelican、Prism/Astra、Mihomo、Regional、额外provider媒体 | 范围/架构待确认；存在Fork相邻模块不代表上游整包等价 | 必须按产品合同单独界定，不能为“同步”自动扩新运行时/新服务 | 各自保留明确阻断/待决，不能静默排除或虚报完成 |
| 发布交接 | #398精确部署、health、服务状态和未认证401权限回归已核验；数据库账本沿用#397只读证据 | #398质量与#396 Key/倍率管理员验收仍待完成；下批累计后再PR/CI/部署 | 不以CI或合并代替部署监督，不因文档小变动单独重启生产 |

## 状态定义

- **Fork 等价**：当前生产已有行为，且有对应代码/测试证据；仍需按风险做生产抽样。
- **已部署**：精确生产 SHA 和部署记录可核对；不代表管理员业务验收完成。
- **候选待发布**：本地代码和门禁通过，但生产未应用，不能对外宣称可用。
- **未完成/阻断**：缺少功能闭环，或存在隐私、数据、权限或依赖风险；不能用页面占位或文档替代。

## 上游全版本关键功能索引

来源范围是共同祖先 `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a` 到 tag `9620d315f72a7dbf04fa10fca02970e1e410805e`。以下为已发现的重要功能簇，不是 446 个 first-parent merge 的逐项关闭证明。未出现于表中的 merge 仍是审计未结项；不得仅凭文件路径差异推断是否纳入。

| 上游功能簇 / 关键 PR | 上游证据锚点 | Fork 当前证据 | 分类与后续 |
|---|---|---|---|
| BPS/Excel OAuth：#346/#373/#374/#379/#506/#541 | Excel OAuth、BPS runtime/reauth、Excel account capability | production 未发现同一 OAuth provider/runtime 链 | 需先定产品范围；若纳入，作为 OAuth、网关、刷新恢复和能力权限的完整工作包 |
| Token guard/自动重登/凭证失败：#185/#440/#557/#560/#574 | `account_token_guard_v2.go`、reauth runtime、迁移 254-257/262/300/309 | Fork 已有 v2 guard、reauth worker、failure reader/writer/API/UI 与对应候选测试；生产迁移和行为差异仍需逐项复核 | 主要能力已有但未完整等价；历史reader/structured marker已随#395修复，网络timeout分类与管理员动作继续核验 |
| 质量隔离/恢复/Jev：#51/#174/#437/#447/#522/#528/#548/#550/#565/#575/#576 | quality ownership、rule template、judge、live regression 与恢复审计 | 已部署scheduled tests/outbox/ownership/Jev/API/UI；具体缺口见上表，不再笼统列为全缺 | 部分等价；避免平行重造，按缺失合同扩展现有模块 |
| Priority/latency admission：#315/#422/#508/#520/#572 | timing details、physical attempt identity、HTTP/WS latency admission、penalty clock | Fork 已部署优先调度页/API、硬准入和attempt持久化；#395已接入单模型quality round；本批补多模型完整round、逐模型归因及原生WS首语义观测 | 最终admission与attempt持久化已部署；生产管理员验收及部署后真实WS观测行核对仍待完成 |
| 用量/模型/认证：#413/#432/#490/#504/#562/#567/#570/#577/#579 | usage cache、GPT-6.1 Sol、response model、connection diagnostics、token rotation、traffic records、group multiplier、rolling window | GPT-6.1 Sol 有部署实现/测试；#579 精确滚动窗已随#393上线；#562 token rotation 已有 Fork 等价实现；#479 multi-group API key已随PR #396部署、schema核验通过但本机验收待完成；#567 user traffic、#504 connection diagnostics和#570 model/group multiplier仍待范围确认 | 混合状态；不把 QPS/请求台账当字节流量，不把现有基础倍率当 #570 等价；逐行为审计 Fork billing/auth contract 后分类 |
| 协议/媒体/供应商：#448/#449/#457/#459/#460/#468/#473/#485/#506/#542 | Pelican、模型广场、image scheduling、Grok realtime/TTS、MiniMax video、OpenAI SIWC、additional models | Fork 有独立 provider/capability/catalog 路径；SIWC 等路径未发现直接等价证据 | 需按当前支持 provider 和真实用户流量确定纳入，并逐协议检查认证、转发、计费和前端能力 |
| 请求采集/支持/平台运行时：#77/#82/#143/#303/#334/#385/#391/#399/#419-428 | request capture、Mihomo manager/static proxy、monitor V3、support tickets、Astra、regional runtimes/settlement | Fork 没有相同完整运行时；部分依赖多实例协调、守护进程、区域节点或独立服务 | 范围/架构待定；未有依赖基础前不得当作简单 UI 移植或声称已明确排除 |
| 安全/IP 风险/审计：#445/#487/#492/#513/#523/#525/#529 | auth IP guard、IP risk、prompt penalty、abuse block、evidence retention、fingerprint block、security workflow | Fork 有自有 IP risk/security audit/prompt audit；语义和留存/权限范围不等价 | 需逐合同适配；#492 全文保存和跨用户详情无 TTL，保持隐私阻断，不原样迁入 |
| 管理和生产交付：#479/#493/#516/#519/#556/#563/#566/#573/#580/#581 | API key 多组、cookie/session、timer、moderation recovery、audit categories/locales、channel ops、release identity | 请求台账 admin-only 已生产；渠道运维已部署；生产#393/#394已有moderation recovery/滚动24h；独立启用为Fork审核记录行入口 | 按 Fork 已有实现做增量对照；已部署基础功能，写动作现场管理员验收仍待完成 |

本索引帮助避免漏掉早期功能簇，但不能替代“每个相关 merge 被某个功能簇引用”的覆盖审计。集成负责人需产出 merge SHA → 功能簇映射，并给每个最终功能域附 Fork 路由/服务/迁移/测试证据，再将结论置为“等价、Fork 适配、缺失、明确排除、隐私/架构阻断”之一。

本矩阵当前结论是 `fully_migrated=false`。任何状态变更必须同时更新代码证据、测试、生产证据和回退条件。
