# 普通签到 HTTP 成长字段修复

范围：`FORK-PLAY-003`。只把服务已返回的 `GrowthEnergy`、`GrowthEligibility` 投影到普通签到 DTO，并纠正其注释。补签、资格规则、现金数额、奖池、余额账本、计费、迁移及页面实现均不修改。

初始审查基线：`a7e1c00330170754683216e3018cbd4279d7f733`。工作分支：`codex/checkin-growth-contract-20261009`。独立 worktree：`/workspace/checkin-contract`。

验证期间主线程合并 PR340；本分支通过普通 `git merge origin/play/main` 快进到 `4080e2ac7e93dc9f435e0c0a4652834635cba7be`，没有 rebase/强推，保全所有本任务改动。最终验证使用该基线的全量迁移（包括 273/274）；上游核心分支未修改。该基线的生产部署/验收状态由主线程单独核实。

## 复现与回归证据

RED：真实普通签到 handler 服务成功写入能量后，响应不含 `growth_energy`，`growth_eligibility` 是空值。隔离 PostgreSQL 完整迁移后，正式路由 → 服务 → 仓储/事务写入签到、资格快照、能量流水各一条；余额不变。真实 Axios 客户端驱动现有 Vue 页面，中文提示实际为 `已到账 $0.00`，期望为 `签到完成，获得 1 点成长能量`。现金页面用例同时通过。

测试入口：

```sh
cd backend
go test ./internal/handler -run '^TestCheckinHTTPProjectsGrowthReward$' -count=1 -v
cd ..
./scripts/test-checkin-contract.sh
```

专用脚本强制 `CI=1`，默认 Testcontainers 模式下缺失 Docker 必须失败，并强制执行前端实时 HTTP 用例。数据库默认采用仓库 integration harness 的一次性 PostgreSQL/Redis；新基线也允许显式启用、严格验证回环地址/专用测试用户和库名的 local external 模式；本任务及 PR CI 使用默认容器模式。不接受生产账号，不访问生产或付费 API。`ProvidePlayService` 与正式路由注册保持当前生产配置，包括资格规则和不启用旧 cohort 审批门禁的个性化。

闭环矩阵：

| 情形 | HTTP / 前端 | 数据库断言 |
| --- | --- | --- |
| 已验证、无近期活动 | energy=1、none、完整资格；中文能量提示 | 一条签到、一份快照、一条能量流水；现金 0、余额 1 不变 |
| 邮箱未验证 / 账号未满三天 | 对应资格原因，继续给予非现金能量 | 快照实际资格信号逐字段匹配 HTTP；仅一条能量 |
| 有资格现金奖池 | balance=0.5；旧金额字段和 streak 保留，energy 零值仍省略；中文 $0.50 | 签到与资格快照关联；奖励账本/余额流水各一条，余额 1→1.5 |
| 并发两次 + 顺序重试 | 恰好一次 200，其余 409/PLAY_CHECKIN_ALREADY_DONE | 再次查询各记录数量/金额不变 |
| 关闭功能 / 未认证 | 400 / 401 | 零签到、零快照、零奖励，余额不变 |
| 刷新 / 重新挂载 Vue | 真实 GET 返回 checked_in_today，按钮禁用 | 重试后重新对账无重复奖励 |

前端替换认证 store、外层布局、路由跳转、分析埋点和 toast 收集，不模拟 `playAPI`、Axios、handler 或奖励响应。此用例验证真实 HTTP 驱动的组件行为；不是浏览器视觉/生产验收。默认纯前端测试会显式跳过两个实时用例，独立 PR `Check-in PostgreSQL HTTP and frontend contract` 作业通过上述脚本强制执行。

## 审查与边界

独立规格审查要求补充 HTTP 资格与持久化快照逐字段一致性断言，已补齐并复审通过。独立质量审查通过；运行门禁结果及准确交付 SHA/PR CI 见 PR 记录。

全量集成门禁发现既存的模型媒体能力迁移测试断言错误：视频用例期望 2/3 种分辨率，却查询 `image.supported_sizes`，实际得到 0。在未修改的 `4080e2a` 独立 worktree 上复现同样失败后，仅修正测试按模态读取 `video.supported_resolutions` 或 `image.supported_sizes`，保留原数量期望。该测试修正单独提交，不修改迁移或模型业务；修正后的定向集成测试通过。

2026-10-09 在 `4080e2a` 基线上的最终服务器门禁：

| 门禁 | 结果 |
| --- | --- |
| `make test` | 通过；Go lint 0 issues；前端 489 文件 / 3465 测试通过 |
| `make test-backend-unit`、`make build` | 均通过 |
| `CI=1 CHECKIN_FRONTEND_CONTRACT=1 go test -tags=integration -count=1 -json -timeout=10m ./...`（backend） | 通过；15939 个测试/子测试通过，55 个含测试包通过；核心迁移要求的 43 项全部通过 |
| `./scripts/test-checkin-contract.sh` | 6 个子测试及父测试通过，真实 HTTP 前端 2 项通过，无跳过 |
| `./scripts/check-fork-integrity.sh`、前端 `design:verify`、文档链接检查 | 均通过 |

默认前端全量命令跳过的两个实时用例由专用脚本实际执行。全量 Go integration 另有 20 个既存测试/子测试因专用环境、外部服务或显式 sentinel/TODO 条件跳过，涉及 prompt audit、TLS capture、付费 API、插件等，未将它们计为通过；本任务的数据库闭环与核心迁移 43 项没有跳过。GitHub CI 与最终 SHA 另见 PR 记录。

已确认但留待下一批的既存错误展示问题：`CheckInView.vue` 从 `err.response.data.code` 读取业务错误，`api/client.ts` 实际返回顶层 `status/code/reason`。过期状态/多标签页下的重复签到会落入通用失败，无法执行专用刷新分支。本 PR 的闭环范围是成功显示、HTTP 错误契约和数据库幂等，不宣称页面所有失败提示已验证。

交付到 draft PR，禁止本任务自行合并、部署、生产签到、补发历史奖。最终部署健康和本地浏览器验收由主线程协调。仓库引用的 `docs/PROJECT_HYGIENE.md` 在初始基线缺失，执行已存在的根 AGENTS 和 DELIVERY_WORKFLOW。

后续玩法/福利全量验收盘点见 [闭环验收清单](./welfare-followup.md)；该清单为后续工作计划，不代表已完成审计。
