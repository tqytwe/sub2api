# P0 请求台账精确关联

状态：实现、独立评审与隔离技术门禁通过；GitHub CI/生产部署与用户本机验收待交接。基线 `726289bdb757e572eac10ab5f12c689c4dc087b5`，
分支 `codex/p0-ledger-correlation-20261010`，隔离工作树 `/workspace/sub2api-p0`。
固定上游来源不变；本任务不合并、不部署、不访问生产凭据，不恢复或推算历史损失。

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
