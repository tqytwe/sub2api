# P1 签到页面错误契约

基线：`726289bdb757e572eac10ab5f12c689c4dc087b5`，远端 `play/main`。
分支：`codex/p1-checkin-errors-20261010`；worktree：`/workspace/p1-checkin-errors`。
范围：`FORK-PLAY-003`；沿用 #342 已完成的成长字段映射，不重做字段修复。

## 修复与 TDD

`api/client.ts` 在真实 HTTP 失败时输出顶层 `status`、数字 `code`、字符串 `reason`。
普通签到与补签此前从 `response.data.code` 取错误，专用分支不可达。两处复用现有
`extractApiErrorCode`，按 `reason` 优先识别，不改公共客户端或服务端合同。
补签的不可用/并发已完成分支复用现有中文提示并刷新状态，移除过期补签入口。
模板、样式、价格、资格规则、奖励数额、schema、SQL275、路由及计费均未修改。

RED 证据见 [red-evidence.txt](./red-evidence.txt)：一次性 PostgreSQL 完整迁移后，正式
Play 路由、ProvidePlayService、仓储、事务和流水实际执行。两个 Vue 实例先读未签到
状态，一个发奖后，另一个以过期状态提交；四个实时前端用例因专用提示缺失失败，
同次 PG 对账均通过。修复后四个用例通过，没有手工替换 Axios 错误对象。

HTTP 故障矩阵仅在回环 HTTP 服务端注入 wire response 或保持连接触发真正 Axios
timeout；`playAPI` 与客户端拦截器不 mock。它覆盖现有业务原因和未知原因、补签拒绝、
超时恢复及同一 tick/禁用期间重复点击。历史治理错误只做兼容提示回归，不恢复已停用审批规则。

## 验证边界与矩阵

| 场景 | 实际验证 | 持久化核对 |
| --- | --- | --- |
| 成长能量、现金 | 真实 HTTP→Axios→Vue 中文成功提示；过期标签页 409；刷新禁用 | 一次签到/快照；能量或现金流水各一组，无重复奖励 |
| 邮箱未验证、账号过新 | 真实 HTTP 返回资格原因及能量 | 资格快照与响应逐字段一致；现金零、余额不变 |
| 普通签到七天里程碑 | 并发一次成功；Vue 显示合计及里程碑；重复提示及刷新 | 日奖 0.50、里程碑 1.00，两组奖励/余额流水；一份关联快照；余额 1.00→2.50 |
| 补签七天里程碑 | 并发一次成功、顺序拒绝；两个实际 Vue 实例、刷新移除补签 | 合计 1.50，一组补签奖励/余额流水；昨日日期、streak=7、快照关联一致 |
| 关闭功能、不可补签、未认证 | 真实路由 HTTP 拒绝 | 零奖励、零快照、余额不变 |
| 业务错误、未知错误、超时 | 回环 HTTP 故障注入，经真实客户端的 14 个 UI 用例 | 不将故障注入用例宣称为 PG 场景 |
| 相邻福利页面 | PlayHub、Blindbox、QuizQuest、Arena 现有单测回归 | 未增加这些页面完整 HTTP→PG 验收，未称其全状态通过 |

所有金额均是隔离 fixture 的确定值；没有生产奖励、真实支付、历史补发或资金估算。
测试身份通过合成账号 allowlist 替换认证；401 只证明该测试身份门，不能当完整 JWT、
管理员权限或生产三身份验收。后端完整门禁另覆盖仓库已有权限测试。

## 命令与证据

```sh
./scripts/test-checkin-contract.sh
pnpm --dir frontend exec vitest run src/views/user/__tests__/CheckInView.errors.spec.ts
make test
make test-backend-unit
make build
cd backend && CI=1 CHECKIN_FRONTEND_CONTRACT=1 go test -tags=integration -count=1 -json -timeout=10m ./...
```

构建后浏览器证据使用真实 Vue production assets 与回环 HTTP fixture，浏览器外部请求
全部拦截；仅原型通过现有 store 主动展示预期 Toast，不作为修复测试。基线和更新测试
均由用户动作触发真实 HTTP 响应。截图等待 Toast 进入动画结束再捕获，未修改显示文案。
见 [视觉记录](../../../visual-reviews/2026-10-10-checkin-errors.md) 和本目录
`browser.py`、`baseline-browser.json`、`updated-browser.json`。

最终测试数量、独立规格/质量审查、准确 head SHA 与对应 GitHub CI 以本批 PR 为准。
默认前端命令会明确跳过四个实时用例；专用脚本和本次完整 integration 显式运行它们。

完整 integration 的最终重跑为 16,660 个测试及子测试通过、23 项跳过、58 个测试包通过；
66 个包无测试文件。跳过原因逐项记录在 [integration-results.json](./integration-results.json)，
包括未启用的其他模块 fixture、外部服务/密钥条件及仓库已有跳过项；没有跳过签到合同。
首次并行完整门禁遇到容器 OOM，`make test` 与 integration 失败，随后停止该次 integrity。
最终完整重跑将重型任务串行执行，并使用 Go 并发 2、`-p=2`、`GOGC=50`、`GOMEMLIMIT=3GiB`；
首次失败不计作通过，没有为消除失败而改动被测业务。

最终本地门禁结果及源码 SHA256 见 [validation-results.json](./validation-results.json)：
`make test`、`make test-backend-unit`、Fork integrity 均退出 0，完整构建通过；
前端全量为 493 个文件 / 3,520 个用例通过，4 个实时 PG 用例仅在默认命令中跳过。
这些记录证明本分支技术验证通过，不表示已合并、已部署或所有福利页面验收完成。

## 独立审查

2026-10-10 按仓库顺序完成两轮独立只读审查：规格审查先要求补齐实际构建资源的
pending 重复点击证据；普通签到/补签 12 场景矩阵实跑通过后复审通过。随后质量审查
通过，未发现本次 diff 引入的阻塞缺陷，确认错误提取、异步恢复和 PG 并发对账边界。
审查者没有替代主执行者运行完整门禁，也没有把本地 fixture 认证或截图称为生产验收。

## 后续与限制

- 已复现的独立缺口：状态 GET 503 被现有页面显示为“签到功能暂未开启”；这次只修提交错误契约，保留此缺口。
- 里程碑提示的目标天数/剩余天数口径，以及优惠券/兑换码与里程碑同时发放时的页面说明，需要另立用例核查；不把本次现金里程碑通过外推至所有奖励类型。
- 玩法、答题、盲盒、竞技场、战队、邀请活动及管理员面继续按 #342 的福利清单分批验证；没有修改或宣称这些页面完整闭环。
- 不合并、不部署、不访问生产；用户本机三身份及生产 UI/API/DB 验收待主线程另行协调。
