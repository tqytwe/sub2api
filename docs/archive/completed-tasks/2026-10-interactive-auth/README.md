# 交互登录响应归属修复

状态：独立分支 `codex/p1-interactive-auth-20261011`，基线 `1b701237ec3ceb869c39d28ad11d32e4065a2471`（主线程普通合并 #358，已 fetch 核实）。发布前已按主线程授权 fast-forward 合入最新主线 `fdec971d2a01c25d93874b7688dffcff78f79088`（#361）；后端/迁移/CI与主线完全相同，六项原冻结源码及构建前端资源不变，见 [组合证明](combination-proof.json)。独立草稿和精确head/CI以PR为准，不合并部署、不访问生产。

## 问题与范围

迟到的 login/login2FA/register 失败无条件清除后来会话，迟到成功还会先由 API helper 持久化旧凭据。较新请求仍在途时，Axios 的旧401也会在 store 处理之前清除现有状态。前批三版本都复现的旧问题证据保留于 [#358归档](../2026-10-auth-refresh/README.md)。本批不修改已冻结的 #358 分支。

生产范围仅 `api/auth.ts`、`api/client.ts`、`api/tokenRefresh.ts`、`stores/auth.ts`。增加当前文档交互请求序号，结合既有generation/userID/sessionID守卫 API持久化、Pinia采用和失败清理；只有当前请求可发布或清理。请求元数据让旧401在Axios清理前失效。开始新交互时先协调已观察到的peer存储变化，再捕获归属，保留既有未完成发布恢复能力。正常令牌旋转不取消当前登录。正式会话身份接口语义、权限、令牌期限、账户政策、页面结构及文案不变。

## 实际验证与边界

真实Pinia、API helper、Axios拦截器及Node回环HTTP，未mock错误对象。初始77例61失败/16通过；第一轮177例173通过/4失败（3旧401清理及1旧mock参数断言）；修复后200例通过：新交互100、既有auth HTTP39、store24、client30、coordinator7。新增恢复路径曾1失败/99跳过，修复后纳入100例全绿。取消使用真实Axios AbortController，超时使用真实HTTP挂起和传输超时。

覆盖旧200/401/503、取消/超时、后续切号/同账号重登/注销、新请求未完成或先失败、2FA challenge、同账号返回相同token、正常续期及setToken替换。对照检查当前请求的错误状态及原message/reason合同、普通登录及恢复。账户与令牌都是确定性合成夹具，不验证生产JWT签发、真实TOTP、WebAuthn或生产账户政策。

规格审查 PASS 后顺序完成质量审查 PASS，独立真实 HTTP 探针分别3/3、4/4；见 [审查记录](reviews.md)。本轮门禁逐项记录如下，不能使用旧 #358 CI替代。本批正式矩阵限定login/login2FA/register；Passkey认证器及直接OAuth回调未按该矩阵验证，不宣称全部认证竞态已消除。已知局限与最终结果见后续验证记录。

## 构建浏览器证据

本轮 `make build` 已通过，最终静态入口 SHA256 为 `f3d9d20d8ea1766cd5c02e22a218a473f1e63cceb9b2ef4b43e6b419c6872e6f`，全资源清单见 [built-resources.json](built-resources.json)。原生 Chromium 直接调用实际构建 Pinia action，经实际 Axios 与回环HTTP：三种认证入口 × 换号/同号重登/注销 × 旧200/401/503，共27/27通过；先确认目标新身份、新会话已采用或注销完成，再对照完整存储及Pinia，均拒绝旧结果、无JS错误和横向溢出，不做后续手动恢复。该脚本不点击登录表单，不验证真实2FA认证器，截图仅记录当时载入的页面。见 [脚本](browser-interactive.py) 和 [报告](browser-interactive-evidence/report.json)。

答题与盲盒主动续期/401自动续期的4项原生构建回归通过，覆盖360深色/1280浅色及中英文，未手动恢复Pinia，幂等key重试保持；见 [报告](browser-refresh-evidence/report.json)。构建浏览器使用回环合成HTTP，不是生产JWT或HTTP→PG证明，PG契约另跑。

新浏览器夹具前三次分别因阻塞Playwright路由回调、快照表达式括号、未等待Vue挂载失败；均发生于测试脚本，修正后27项完整重跑通过。原始失败和TDD日志保留于 [test-evidence](test-evidence/manifest.json)，不将失败运行的部分用例计入通过总数。首次全量门禁因临时目录ENOSPC失败，保留日志；恢复环境后重试结果单列。

跨标签8项回归通过，覆盖阅读时续期保留答题选择、双页只旋转一次、换号/注销/同号重登、旧请求在途与缺失storage事件的回收；原生Web Locks未包裹，实际既有profile定时器参与。见 [跨页报告](browser-cross-tab-evidence/report.json)。新增证据经独立规格→质量补审PASS；首版缺少目标状态断言的证据已补强并完整重跑，未变更生产源码。

隔离PG/Redis容器的福利与签到契约均通过，真实正式路由→handler/service→PG再经真实Axios/Pinia/Vue，覆盖答题和盲盒的现金/能量/券/兑换码（按各功能实际支持）、重复与并发幂等、权限、余额不足及事务回滚；签到覆盖资格拒绝、补签、里程碑与前端HTTP。测试身份中间件与确定性奖池为合成夹具，不是生产JWT/真实随机概率验证。记录见 [福利PG](test-evidence/welfare-pg.txt)、[签到PG](test-evidence/checkin-pg.txt)。未更改数据库结构、后端代码或奖励规则。

## 全量门禁及组合增量

`make test` 首轮ENOSPC失败；恢复后后端default测试/lint、前端lint/typecheck通过，前端全量3743通过/1失败/8跳过。唯一失败是`LoginForm.spec.ts`旧mock严格单参数断言；真实helper现在另收内部归属参数。仅将其改成调用一次且首参数业务载荷严格相等，导航/错误等原断言保留，定向5/5通过。本轮没有宣称该失败的`make test`进程exit0；按主线程加速指示，不重跑未变源码且已通过的耗时用例，精确新head完整CI另核。

后端tag-unit首次只有既有`TestTokenRefreshService_SaturatedProviderPreservesConcurrencyAndActualQPSStartSpacing`失败（并发运行下0.13ms低于5ms断言），其余包通过；单独连续10次通过，原因未最终确证。组合后的service失败包和requestledger已完整复核通过，结果见 [验证记录](verification.json)。未修改该后端实现或测试。

`make build`通过；Fork首次链接ENOSPC失败后完整重跑通过。合入#361后后端组合构建通过，复用输入完全相同的前端356文件与原生浏览器/PG证据。新增LoginForm断言增量另作顺序独立复核。环境失败、旧mock失败与QPS失败全部保留，不把同一失败进程改写为通过。

## 根审后续

主线程在c4d0e54c发现一次性存储失败导致自身publication未清理的新增回归，该head不可合并。最小所有权修复及新的RED→GREEN/构建资源/顺序复审另记[root-storage/README.md](root-storage/README.md)，不覆盖原c4证据。
