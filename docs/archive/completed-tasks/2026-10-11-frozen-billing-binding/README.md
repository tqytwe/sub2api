# 冻结计费目标绑定修复

来源：root 对已复现 A→B 归属证明错配的修复授权。基线 `05b44bec5c2d30ad58d009fbe5247ea363c6ee70`；分支 `codex/p1-frozen-entitlement-binding-20261011`；worktree `/workspace/sub2api-p1-binding-fix`。关联 FORK-BILLING-010 / FORK-REQUEST-LEDGER-020。

生产范围仅 `backend/internal/requestledger/billing.go` 的 Prepare/Verify 绑定验证。既有 fingerprint 算法、其他结算/撤销规则、历史金额及数据不变。未声称生产自动重选套餐或发生重复扣费。用户已授权“撤销立即阻止新请求，撤销前获准的在途请求按原套餐和冻结价格结算”，但该规则须在后续独立补丁先完成方案/兼容评审，本补丁不实现也不宣称解决；精度审计同样独立后续。

## 实现前兼容合同

| 情况 | Prepare | Verify（同一 billing Tx） | 保留语义 |
| --- | --- | --- | --- |
| 新请求，无 dedup，首次固定目标 | 建立 link | 同一 billing Tx 内匹配 fingerprint proof，认证随 Tx 一起提交 | 正常首次计费 |
| 已有 link，fingerprint、subscription/entitlement 均精确一致 | 幂等通过 | 通过 | live/archive 原命令 Applied=false，不重扣 |
| 已有 link，任一 ID 不同、NULL→非 NULL 或非 NULL→NULL | 拒绝 | 拒绝，整笔 Apply 回滚 | NULL 不作为通配，不改旧 link |
| 已有 link，两边同为 NULL（含普通订阅 entitlement=NULL） | 通过 | 通过 | 保留原有未知/不适用字段，不认证新 ID |
| 旧 dedup 已存在，但此请求缺少 link；传入任何非 NULL 目标 ID | 拒绝新插入 | 缺 link 时拒绝 | proof 只有旧 fingerprint，不能据当前输入认证历史目标 |
| 旧 dedup 存在、缺 link，且本次两个目标仍均 NULL | 允许建立两个目标均 NULL 的未验证 link | 仅在 link 存在且 fingerprint/proof 相同时认证 | 不增加目标归属声明；保持无目标的钱包/历史路径 |
| 无 ledger context | 保持原 no-op | 保持原 no-op | 不扩大请求台账覆盖范围 |

旧 NULL / 无 link 的退化是明确的：不能凭当前套餐、请求传入值、当前价格或 old fingerprint 补出历史 entitlement。旧 link 与非 NULL 输入不一致时返回现有 `ErrUnavailable`，不回写、不补扣，后续仍需可信原始证据；这不是任意历史请求都能修复的承诺。缺 link 且有目标的旧 proof 不能新建可信 link，即使调用者声称这些就是原始 ID。正常历史重放指已有原 link 与原上下文的精确重放，不把缺 link 的重建解释为正常证明恢复。

## TDD 与证据范围

- `subscription_binding_reproduction_integration_test.go`：真实 HTTP→RecordUsage→完整 PG Apply 故障回滚后内部 A→B，合法 A 恢复，以及直接 Verify live/archive 对照。
- `subscription_binding_compatibility_integration_test.go`：完整迁移 PG，live/archive × 17 项 exact/NULL/不同ID/不同fingerprint/缺link矩阵；旧 proof 的键、fingerprint、created_at 与 link 全行 snapshot 不变；精确 Apply 重放；无 ledger context；两个不同 ID 同时 Prepare 只能一个固定成功。
- 只在 disposable fixture 内构造旧 NULL、缺 link、archive；不对真实历史行做迁移或修改。
- 使用原 `scripts/test-subscription-counters.sh`，不同 top-level suite 独立启停 PG，不能复用已迁移 DB。

## 当前验证

兼容矩阵先于实现运行，基线为 13 PASS / 17 严格断言 RED。首次实现运行因新 SELECT 的参数类型问题未通过，已显式限定 PostgreSQL 参数类型并重跑；未放宽断言。

最终三套独立 disposable PG / 404 全迁移 / `-race`：兼容矩阵36/36、原 A→B 与直接 Verify 复现4/4、既有计数器/归属矩阵42/42全部通过，无 race 诊断。兼容矩阵初版30/30之后，仅补六项 NULL-only/单 ID 缺 link 分支对照，再跑36/36；生产实现未变。原旧指纹、live/archive proof 的键/created_at 和历史 link 快照保持一致；原指纹算法未修改。命令、时间、exit code、原始日志及 SHA-256 见 [targeted-evidence.json](./targeted-evidence.json)。原 A→B 的基线 RED 复用同05b基线的隔离调查日志；最终测试仅将两处 deferred Rollback 改为显式忽略清理返回值，断言未变。清单分别记录旧、新测试源码哈希及复用说明。审查清理修正后再次独立运行36/36、4/4 `-race` 并通过。

可读 `.txt` 日志仅展开 tab 并清除行末空白，以满足 diff whitespace 门禁；原始字节另存 `.raw.txt.gz`，清单分别记录两种 SHA-256。断言、错误文本和运行结果未修改。

[顺序规格审查、质量审查](./reviews.md) 均 PASS；审查发现的三处 Rollback 清理包装问题已修正并复审。以下本地门禁全部 exit 0；原始日志、命令、时间和 SHA-256 见 [gate-evidence.json](./gate-evidence.json)，工具版本见 [environment.json](./environment.json)。

| 门禁 | 结果 |
| --- | --- |
| `make test` | 后端默认测试和 lint 通过；前端499文件/3601测试通过，既有3文件/8测试跳过 |
| `make test-backend-unit` | 带 unit 标签的后端全套通过 |
| 台账 PostgreSQL/protocol integration | 445项通过，2项预期夹具跳过；CI要求的48项全部通过，见 [用例清单摘要](./ledger-case-summary.json) |
| `make build` | 后端和前端完整构建通过 |
| `./scripts/check-fork-integrity.sh` | 通过 |
| 新增范围 integration/billing_acceptance tagged lint | 通过 |
| 文档链接、diff whitespace | 通过 |

台账两项跳过为子进程专用 fixture 与可选浏览器 fixture；实际父进程 kill/recovery 等48项必需场景已执行。没有把跳过项写成通过。

当前限定修复的本地代码与回归已验证，等待草稿 PR 对应 head 的完整 CI 及主线程审查。共享 handoff/Ops/迁移账本按并行约定由主线程关联本 PR/SHA，不在本分支修改。未合并、未部署，未访问生产或执行付费生成；本记录不代表用户本机三身份或生产对账验收完成。
