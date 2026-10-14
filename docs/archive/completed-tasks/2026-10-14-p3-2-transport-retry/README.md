# P3-2 transport-retry 移植（v2.10.5）

来源：ranxi2001/sub2api v2.10.5 `gateway_upstream_transport_error.go`（base blob a9f459fe → target blob 0ee5188b）。

## 移植内容

上游语义：直连（无代理）池模式账号若在 `pool_mode_retry_status_codes` 中列出 503，
则传输层失败（dial 拒绝 / DNS / TLS，未产生 HTTP 响应）按上游 503 处理——
同账号按请求级退避重试（pool_mode_retry_count 次）后再切换，跳过 10 分钟临时摘除。
代理账号与未启用 503 的账号保留原有摘除行为（代理拨号失败正是临时摘除针对的持久故障）。

## Fork 自适应差异（与上游逐行核对）

- 上游 `IsPoolMode()/IsPoolModeRetryableStatus()` 读 `Extra` 字段；Fork 实现读 `Credentials`
  （`pool_mode`、`pool_mode_retry_status_codes`），调用点同名，语义等价，移植无需改动调用代码。
- 保留 Fork 已有的 ledger receipt 归因（`event.LedgerReceipt = requestledger.ErrorReceipt(err)`）
  与 `requestledger.ErrUnavailable` 提前返回——这些在摘除/重试判定之前，不受影响。
- 判定插入点与上游一致：Ollama 计数之后、持久性摘除之前。

## TDD 证据

- RED：3 个新测试中 `TestHandleUpstreamTransportError_PoolMode503OptInSkipsEviction` 在未实现时失败
  （期望 503，得到 502）；另外两个负面测试（未启用 / 代理）在旧行为下即通过，作为防回归锁定。
- GREEN：实现 `poolModeTreatsTransportErrorAs503` + `poolModeTransportFailoverError` 后全部通过。
- 完整 `go test -tags unit ./internal/service/` 通过；`go build ./...`、`go vet` 干净。

## 门禁说明

- race 检测与 PG 集成测试本地不可用（Windows 无 cgo 页面文件 / 无 Docker），委托 PR CI。
- 无 DB 变更、无前端变更。
