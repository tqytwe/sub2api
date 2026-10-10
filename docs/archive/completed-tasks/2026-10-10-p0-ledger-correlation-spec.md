# P0 请求台账精确关联

状态：e209 初次门禁及本轮追加修复的独立评审、串行完整门禁均通过（2026-10-10 22:12 UTC，仅隔离环境）。GitHub CI/生产部署与用户本机验收待交接。初始基线 `726289bdb757e572eac10ab5f12c689c4dc087b5`，
分支 `codex/p0-ledger-correlation-20261010`，隔离工作树 `/workspace/sub2api-p0`。
固定上游来源不变；本任务不合并 PR、不部署、不访问生产凭据，不恢复或推算历史损失。

## 实施前最小设计

- 复用服务端生成的 `gateway_requests.id`。每次真实上游发送在原 admission
  事务中新增独立 attempt UUID；客户端 request ID、上游 ID 不参与此身份或计费去重。
- 新迁移只创建 attempt 证据表及 Ops 精确引用表，历史 SQL/checksum 不变，
  不回填旧 attempt，不修改或删除台账。旧二进制继续写原五表，新读者显示无证据。
- attempt 证据仅含服务端身份及经过长度/ASCII 字符白名单验证的上游 request/response
  ID。无请求正文、密钥、完整响应或任意错误正文。上游 ID 不是计费依据。
- Ops 事件在发生时冻结不可变引用；WS turn、重试和异步回调不得在落库时重新选择
  “当前/最后一次”attempt。只有可信本地引用可写关联，数据库再次核验归属。
- Ops 引用在诊断原记录已落库后独立写入；写失败只能留下证据缺口，不能阻断、撤销或
  重试真实 usage 结算。保留原 admission fail-closed 及既有 wallet/subscription/价格合同。
- 管理员详情新增 attempt UUID、上游标识、原 Ops 详情入口。普通用户仅返回本人
  attempt UUID，不返回上游/账号/Ops 内部字段。旧记录没有关联时明确显示缺失。

## 涉及契约

| 层 | 增量合同 |
| --- | --- |
| DB | 新表/索引、空证据兼容旧写者、FK 不依赖可清理 Ops 记录 |
| gateway | admission 生成身份，transport/parser 保存仅标识证据；协议失败可覆盖 audit 的 HTTP200 终态，业务计费判断不变 |
| Ops | 事件快照与后台队列保留原引用；无按时间、上游 ID 或客户端 ID 猜测 |
| API | 台账 attempt 增量可空字段；管理员诊断引用；既有 401/403/404 和租户隔离 |
| UI | 复用现有详情弹窗、表格、链接；请求/用量/结算三个状态轴保持独立 |

## 验证要求

先失败测试；真实 loopback HTTP/WS + 一次性 PG 验证成功、失败、取消、重试、
断流、延迟回调、并发和幂等；故障注入证明证据写失败仍可结算。
验证旧写入、迁移重放/锁超时、索引、身份边界和敏感内容排除。
实际 production assets 浏览器证据不代替用户本机生产验收。
独立规格审查后代码质量审查；完整 test/unit/build/Fork integrity 通过才提交推送草稿 PR。

生产部署、三身份用户本机验收由主线程协调，本任务不宣称完成这些步骤。

## 实现边界

迁移 276 新增 2 表/4 索引，与 275 的 5 表/18 索引并存。admission 提交 attempt UUID 后才发送；证据或 Ops 引用写入独立、有界、可失败，不以这些诊断结果决定 usage/billing。标识最多 128 字节，限定 ASCII 格式并拒绝常见密钥前缀；首值不覆盖，冲突显式标记。JSON/SSE 解析窗口有界，不持久化正文。Ops 仅可信进程内 Receipt 可写关联，写入时再次匹配 user/key/attempt UUID。

HTTP 重试、恢复旧响应、body wrapper、transport error、插件、compact/图片与异步回调都携带原始 Receipt；没有 Receipt 的外层诊断只能链接入口请求，不猜 attempt。WS 使用各轮本地 attempt；过滤旧终态后、策略处理与 drain 前采集 text 帧标识，binary 保持 opaque，不把通用 event.id 当 response ID。Realtime 已完成 ID 保留有界 4096 项，容量用尽时不新录关联，但已经接受的轮次继续记录真实 usage。

管理员台账与 Ops 通过精确 ID 双向访问；普通用户仅本人 attempt UUID。Ops 保留期清理后引用仍存在，显示不可用。诊断读取失败显示缺口，旧记录无精确关联不做回填。SmartRetry 外部兼容状态不变，每次物理尝试的 HTTP 状态由各自台账/事件保留。

本补丁不能追溯恢复历史 gpt22 损失，也不能据 unknown 推导免费或据上游 ID 扣费。上游锁定 ranxi v2.10.3，P3/P4 不在范围内。订阅验收任务独立负责 usage_billing_repo.go 的归属校验和耗尽重放修复；本分支不修改该文件。


## 草稿复审追加契约（2026-10-10）

- 服务端 attempt UUID 仍在 admission 事务内同步持久化。只有上游 request/response 标识改为最终一致的 best-effort 元数据：单进程1个writer、256个待处理attempt，每attempt合并首request ID、首response ID、一次冲突；每次SQL最多250ms，行锁用SKIP LOCKED。队列满、连接池繁忙（单连接池直接跳过）、锁冲突、超时或进程退出允许证据缺口。空字段只代表没有持久化证据，不能证明上游未提供标识；不补回猜测关系。
- 物理WS连接维护最多4096个已完成响应身份的SHA256，用于协议帧归属，不能用作计费去重。只读取response.id/response_id，辅助event.id不参与。A完成、B实际发送后才到达的A terminal在usage/证据/终态处理前过滤；新created复用已完成ID是歧义失败。
- 同轮明确响应ID冲突、当前terminal缺少明确ID、完成身份容量耗尽时，保留当前已接受usage，连接退役。HTTP完成后释放；ingress先完成AfterTurn，普通下一轮换连接，严格续链（含dedicated）在下一次发送前明确拒绝；匿名/冲突prewarm在主create前退出。容量标记不会提前关闭正在使用的lease。
- 详情先读原请求/attempts/billing/wallet，新证据独立读取；可选查询最多100ms、至多父context剩余预算1/4。不可用返回明确attempt_evidence_unavailable，不伪装旧记录缺证据，普通用户仍不接收上游/账号字段。
- Ops弹窗的详情和相关错误请求共用打开代际；关闭、重开、切换ID/type和卸载使旧请求失效，迟到成功/失败不得替换当前记录、反链、加载状态或提示。

只读订阅风险另记：PackageEntitlementID未进入现有指纹/prepare验证的静态假设尚未动态复现，不代表重复扣费；下一任务需精准RED验证。本分支不修改订阅实现，不搬入#352补丁、不绕过主线程收到的上传权限拒绝。
