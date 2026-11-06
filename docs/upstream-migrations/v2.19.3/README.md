# v2.19.3 上游差异清单（PR #411 后重建）

状态：`inventory-complete / behavior-review-open`。维护责任：本任务提交者，逐功能负责人待分配。这份清单已完整登记来源范围，仍不是“已完整 fork”。目标正式版本为 `ranxi2001/sub2api v2.19.3`，解析提交 `6f813426b61b8eeba77f374059205cc34d2fa993`；底表比较基线为 `origin/play/main@c302cce4f07b071417d0f42320cc4dde7428c7e9`（PR #411）；当前已核实生产为 PR #412 / `e1626d760cf92349bb68fee5c6537a66e1dcbdc6`，349 增量见下方。来源 release、tag object 和稳定发布状态已于 2026-11-06 在线核对。

## 清单覆盖

[提交清单](./commit-inventory.json) 登记共同祖先 `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a` 到 v2.19.3 的全部 728 条 first-parent 变更：历史 446 条 merge、237 条直接提交，以及 v2.17.7 到 v2.19.3 的 45 条增量变更。旧账本漏掉的直接提交已经补入；版本、文档、DeepSeek/Antigravity 等早期直接修复与后续 PR 不再被隐含排除。

[路径对照](./path-inventory.tsv) 对上游与生产基线的全部路径做固定 blob 对照，共 10,987 条路径：2,688 条字节相同、1,700 条内容不同、2,709 条只在上游、3,890 条只在 Fork。路径数量只定义审计边界；相邻文件、同名路由或同功能关键词都不能直接证明等价或缺失。

[迁移清单](./migration-inventory.json) 登记上游目标中的 396 个数字迁移文件、trimmed SHA-256、命名冲突、候选 Fork 文件、涉及对象和生产核验状态。Fork 已新增 349，下一项须从当前未占用编号继续安排；已部署 SQL 不改写。

## 逐项判断规则

每条功能最终必须落到以下一种状态，并同时有 UI/API/schema/test/部署证据：

- **Fork 等价**：行为合同、权限、错误、持久化、后台任务和回读都有证据。
- **Fork 适配**：能力存在，但按 Fork 计费、品牌、玩法福利或导航合同改造，需补差异测试。
- **部分已有**：只覆盖一段行为，例如已有 UltraFast 服务档位和计费测试，不代表上游账号开关、失败回退和价格快照全有。
- **明确保留**：Fork 自有能力或产品策略，继续保留，不接受上游覆盖。
- **明确阻断**：隐私、架构、账务或部署前置条件未满足，不原样迁入。
- **待核验**：只有源码线索或文件差异，没有完整闭环证据。

`fully_migrated` 保持 `false`，不能用文件计数、CI、健康检查或一次页面打开替代行为验收。

完整增量逐项表见[23 个功能包与验收条件](./feature-queue.md)，机器记录见[功能队列 JSON](./feature-queue.json)。以下阶段只是分组，每次实际交付仍限定一个功能包。

## 分阶段执行队列

按依赖和风险排序，每阶段只开一个可审查 PR，完成测试、规格/质量复审、部署和用户验收后才进入下一阶段：

1. **已部署：鹈鹕旧题干**。PR #412 使用 Fork migration 349 迁入上游 #293；5 项真实 PostgreSQL/Redis 专项、完整本地门禁已通过，全部 PR CI 通过，merge `e1626d760cf92349bb68fee5c6537a66e1dcbdc6` 已部署；Zeabur `6aed7bc803a5d91147655138` RUNNING，349 checksum 与源码一致，三表可清理行数 0。用户本机验收待完成。
2. **质量与优先调度补差异**。先核对质量 ownership 的逐模型释放、恢复、孤立状态，以及 priority 的 hard-stop、attempt、WS/HTTP 观测、失败回退和配置联动；复用已有 `quality_ownership`、`priority_scheduling`、`usage_log_priority_scheduling`，不重造入口。需要真实管理员写后读回和至少一次生产只读观测。
3. **渠道/利润/延迟运维**。逐项核对 URL channel entity、revision/CAS、利润叠加公式、滚动 24 小时、延迟 histogram、queue wait/timing 和空值语义；已有 Fork 渠道运维、dashboard、rolling view 只能算候选。
4. **认证连接与凭证生命周期**。多 OpenAI auth connection、连接快照、affinity、refresh fencing、API Key 无认证连接、凭证超时分类按完整 writer/reader/lease/回退闭环处理；不得用单账号 OAuth 或 Excel OAuth 代替。
5. **供应商和协议兼容**。Mistral API Key/Chat、Anthropic Claude Code mimic、Smart upstream protocol、SSE 安全类型、BPS capability 与媒体能力逐协议验收；每项补认证、流式取消、usage/计费和模型目录证据。
6. **账号性能与共享质量批次**。账号性能阈值、独立暂停/恢复、区域快照、并发、公平遍历、共享质量 batch 的租约/截止/混合版本保护独立建合同；不把普通质量规则或 priority 配置当作等价。
7. **抽奖/邀请与其他玩法差异**。上游 recharge lottery、controls、reel、invitations 与现有玩法福利分开建模，先确认产品是否纳入；不得覆盖现有福利、奖池、账本或用户入口。
8. **隐私/支持/Harvest/区域运行时**。request capture、security penalty 明文全文、support ticket、Mihomo、Astra/Prism、regional runtime 需要先明确数据最小化、TTL、跨用户权限、服务依赖和回滚，未完成合同前保持阻断。
9. **逐提交收口**。对 728 条来源逐项关闭 `待核验`，补齐负责人、PR/SHA、测试、部署、用户验收和回滚；完成前不更新来源锁的 `fully_migrated`。

## 当前已确认的差异示例

- **部分已有**：Fork 已有 UltraFast service tier 解析、请求透传和账务 multiplier 测试；上游账号开关、回退策略、冻结价格及历史兼容仍待核验。
- **部分已有**：Fork 已有质量 ownership/恢复和 priority scheduling 配置、attempt/quality round；上游最新孤立释放、共享 batch 和完整 WS/HTTP 语义仍待核验。
- **候选已有**：Fork 已有渠道运维 CAS、质量 dashboard、rolling 24h 与 OpenAI header timeout 静态配置；上游 URL entity、利润叠加、精确 timing/queue wait 的字段来源和写后读回仍需逐项确认。
- **明确阻断**：上游 security penalty 的明文 `full_prompt`、跨用户管理员读取和无 TTL 持久化不原样迁入；Fork 现有 prompt audit 也不自动等价，需单独隐私合同。
- **明确保留**：玩法福利、签到/农场/玩法运营入口以及 PR #411 的 Fork 导航安排继续保留；上游 lottery/invitations 不覆盖这些能力。

## 文件与更新规则

- [提交清单](./commit-inventory.json)：提交级来源、旧结论、当前分类和下一步。
- [路径对照](./path-inventory.tsv)：固定路径/blob 对照，供逐项审查定位，不作为功能结论。
- [迁移清单](./migration-inventory.json)：迁移 checksum、对象、候选映射和生产状态。
- [覆盖检查](./coverage.json)：数量和不完整状态汇总。
- 每关闭一个功能包，更新这里的分类和对应 PR/部署记录；不删除失败证据，不把候选改写为已部署。

## 快照边界与本次校验

底表固定比较 PR #411 的 `c302cce4f`，不是随时间漂移的实时清单。后续 PR #412 的唯一运行时增量为上游 263 → Fork 349；该项在本页作为 overlay 明确记录，不把其他 728 条条目改为完成。来源锁仍为 v2.17.7 的历史分析锁，本文件单独固定 v2.19.3 目标；正式实施其他功能前应更新对应分析锁，不能用旧锁证明新目标。

数量/集合校验通过：728 个唯一 commit 与 Git first-parent 集合完全一致；41 个增量 PR 全部唯一映射到 23 个功能包；10,987 条路径去重与两端树集合一致；396 个 SQL 文件、20 个新增迁移与目标树一致，所有功能证据路径存在。文档链接检查通过。上述校验不证明 728 项逐行为等价；旧条目待核验状态保留。规格与质量自审发现并修正了 header timeout 静态/动态配置区分、未部署状态误写和遗漏的历史直接提交。

仅文档变更不单独触发生产重启；PR #413 作为草稿审阅，下一项按队列确定后再推进。
