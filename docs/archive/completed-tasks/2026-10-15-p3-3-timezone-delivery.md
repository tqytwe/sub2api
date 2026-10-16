# P3-3 账号请求时区 — 交付备忘

- PR: #372 `review/p3-3-timezone`（`f785b6bf9 + 733e4a22c`）
- 合并：`1748967c5` Merge pull request #372 into `play/main`（`2026-10-15T14:09:55Z`，merge commit）
- 基线：`f087df825`（P3-2 transport-retry 之后）
- CI：9 项 check-runs 全部 success（Protected behavior / Documentation / Release source policy / backend-security / Core migration and repository integration / postgres-http-counters / Check-in PostgreSQL HTTP and frontend contract / frontend-security / Ledger PostgreSQL and protocol contract），mergeable_state=clean 后由负责人合并
- 部署：`origin/play/main` 已更新至 `1748967c5`；Zeabur 健康 `GET /health → {"status":"ok"}`，首页 `GET / → 200`，CSP/Cache 头正常；本地 `play/main` 已 `git pull --ff-only` 同步
- 内容：总开关 `openai_request_timezone_enabled`（默认关闭，5s 缓存 + singleflight，保存失效）、账号 extra IANA 校验、HTTP/WS 全链只替换已存在 timezone 字段、admin catalog、Settings/UI、i18n、visual review

## 后续

- P3-4a（PR #373，`61d84c72` on `review/p3-4a-bps-infra`）已变基至新 `play/main`（`1748967c5`）并重推，等待 CI 调度；CI 绿灯后再合并，合并前不启动 P3-5。
- 本备忘遵循 `DELIVERY_WORKFLOW.md` §5 Zeabur 部署核对：已记录部署 commit、健康与关键 API 证据；最终三身份本地浏览器验收待用户在 `https://www.jisudeng.com/` 完成。
