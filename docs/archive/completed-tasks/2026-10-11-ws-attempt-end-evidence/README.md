# WS attempt 结束证据（后端候选）

基线：`95c308f54e30fb6776b2215b6139fc25fa85b6d4`。工作分支：`codex/ws-attempt-evidence-20261011`。
本批只补未来诊断，不解释固定生产窗口的历史失败原因，不认定漏扣、免费或零费用；不回填旧记录。
主线程保留迁移 282；280 属于 #364，历史 281 原字节保留。主线程负责合并和发布。

## 文件与覆盖范围

生产结束钩子只接入 `request_ledger_ws.go` 与 `openai_ws_v2_passthrough_adapter.go`；不改
`attempt.go`（#364 交集）、路由调度、重试决定、原有 Finish 参数或其执行顺序、usage/wallet 结算、
公开错误和鉴权。新增独立快照、查询与 handler，管理员路由只增加一个 GET。
测试夹具只增 282，CI 保留已有必需用例再追加本批。

`version=ws_attempt_end_v1`，`coverage_version=openai_ws_finish_hooks_v1`，限定：

- 经 `dialWSWithLedger` 的 OpenAI WS 握手 attempt；真实 HTTP status 仍只按原逻辑存在原表，本资源不复制或派生 status。
- 经 `finishLedgerWSResult` 的 OpenAI native/pooled HTTP→WS 与 WS ingress model attempt；阶段只取已有 typed error 或明确调用位置，未知保留 unknown。
- passthrough 的精确本地 turn；首帧写失败立即记录，终态回调与 join 后 unfinished 出口分别记录。
- 经通用 WS finish helper 的 prewarm/control attempt；没有已有判定的字段仍 unknown。

这不是所有 WS 帧、Realtime、Anthropic/Gemini 或媒体协议的完整覆盖声明。
不新增 model 自由字符串，不补解析 usage 对象，不新增 UI。

## 数据与丢失语义

每个 attempt 最多保存第一份成功持久化的结束快照，按内部 request UUID、attempt_no、attempt UUID 三者定位。
结束快照是本地结束观察，不保证原 Finish SQL 已成功，也不证明响应已送达客户端。
仅保存版本、受控 stage/reason/source、terminal 结构、布尔判定与本地捕获时间。
队列项在生产者中复制并编码，最多 4096 字节；不携带请求 context、Attempt、原始错误、header、body、凭据或 billing ID。

- `stage_source` 与 `reason_source` 分开：join 出口的阶段可来自 relay exit，而原因来自 context cause。
- terminal 的 `scope=normalized_ws_forward_result`；白名单事件才是 observed/source=ws_forward_result。
  空或未知事件是 unknown；只有 relay reader 已 join 后的 `UnfinishedTurn` 出口允许 not_observed_in_scope。
  后者不证明原始帧中不存在终态：policy rejection 可发生在解析之前，binary 生命周期也有独立语义。
- `WroteDownstream` 在 wire 中为 nullable `wrote_downstream`；未知为 null。
  relay exit 的值注明 `wrote_downstream_scope=relay_session`，不能归为当前 turn 的送达证据；typed ingress error 才使用 attempt 范围。
  OnTurnComplete 发生在 terminal 写客户端之前，该快照不声称已送达。
- `positive_units_observed` 严格复用现有 `HasObservedUsage()`：input/output tokens 或 image count > 0。
  仅 cache/image token detail 为正仍 false。false 不证明缺 usage 对象、usage 为零或零费用。
  `result_observed` 表示调用处是否有 forward result；`usage_presence` 恒为 unknown。
- 单独 256 项非阻塞异步队列，单 worker、250ms deadline、连接池预留一槽、普通 MVCC 归属读 + 独立表 INSERT ON CONFLICT，不锁原 identifier 行，不新增业务同步 DB 往返。
  队列满、锁、池忙、失败或进程退出可以丢证据；不重试、不影响业务，读取缺失明确 not_recorded/unknown。
  新表第一份成功快照不覆写，旧 identifier writer 不会擦除它。

## 管理员读取合同

`GET /api/v1/admin/requests/:id/ws-end?after_attempt=0`，只接受私有 request UUID，复用原管理员权限。
128 项 seek 分页，先选 129 个 attempt 候选判断 has_more。attempt_id 缺失为 null，snapshot 缺失为 null。
`completeness` 恒 unknown。状态包括：not_recorded（旧、未覆盖、pending、丢失不可区分）、persisted_snapshot、
incomplete（坏数据/归属/枚举不一致）、unsupported_version。不会把缺失输出为“没有失败”。
白名单 DTO 不回传额外 JSON 属性。401/403/400/404 保持现有权限语义，成功 no-store。
查询短超时或不存在新表时只让该可选子资源受控 503，不影响原请求详情。

## 282 兼容与迁移

`282_gateway_ws_attempt_end_evidence.sql` 仅新增 `gateway_request_ws_end_evidence`，每 request/attempt 最多一行；
精确 attempt UUID、默认 `{}` snapshot 与 object/4096 字节 CHECK。外键到原 attempts 并随原生命周期级联删除，
不新增历史清理任务。创建前取得 attempts 父表 ACCESS EXCLUSIVE NOWAIT，锁 50ms、语句 500ms；
失败整事务及 migration registry 回滚，可解除争用重试。不扫描或改写历史行，旧 writer/reader 不依赖新表。
应用回退保留新表与已有证据；禁止单独删除、清空或回填。
NOWAIT 不能消除取得锁后实际 DDL/提交的短暂开销。

## 验证记录与限制

实施中的验证状态见本目录后续记录，未完成项不冒充通过。
早期同一 evidence 行的新列方案在真实 race 与确定性受控写阻塞测试中导致原 identifier 丢失，质量审 P2 阻断；
已改为独立表，不把这个失败当作允许的可选 WS 证据丢失。原失败与独立表并发验证分开保留。
首轮 WS 夹具误配置了 pooled dialer，却遗漏 passthrough 独立 dialer，导致四次合成无效凭据的默认上游握手，均 401、零模型帧；
该首轮失败不算本地 WS 证据。夹具随后同时固定回环 dialer 与回环 base_url，后续测试仅使用临时本地上游和 PG。
迁移首轮遇到临时磁盘不足；仅清理可再生成 Go 缓存，未清理数据库卷，保留失败并重跑。
不曾访问业务生产数据库/站点、不收费探测、不合并部署。服务真实回环验证、完整门禁、独立双审及最终 CI 将分别记录，
本批无 UI 接线和用户本机验收。
