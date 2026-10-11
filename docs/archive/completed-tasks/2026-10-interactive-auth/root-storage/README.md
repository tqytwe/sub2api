# #362 根审：中途存储失败的 publication 所有权

历史审查 head `c4d0e54c1a44570b4815a78e0bb148f2e50d93d1` 新增回归：API/Pinia发布已推进会话身份，逐键写入抛错时attempt快照尚未推进，store catch把自己的失败当作过期请求，留下pending/部分凭据。此head禁止合并。

## RED 对照

[原始总结](summary.json)及[临时探针](probe.spec.ts)固定的是修改前状态。真实Pinia、API helper、Axios及回环HTTP，仅让指定Storage.setItem一次性抛QuotaExceededError，后续写入正常。三入口的API/Pinia两次发布、四个凭据字段及ready标记共30例，加3例未登录起点：base fdec清理33/33，c4清理0/33；后续正常登录两版均33/33恢复。另3例显式拒绝传递屏障让真实新HTTP登录先完成，c4保留3/3，base旧无条件清理0/3。屏障是调度放大，不是真实浏览器时序证明。

原生c4编译资源9例同样清理0/9、后续恢复9/9、JS错误0；仅覆盖三个入口的API/Pinia auth_user写入及未登录起点。见[browser-c4-evidence/report.json](browser-c4-evidence/report.json)。这些是loopback合成账户与实际构建store动作，不是表单/TOTP/JWT/生产验收。

## 修复与验证边界

三个公共文件限定：tokenRefresh显式登记本attempt创建的pending publication marker；失败清理必须满足交互序号仍当前，并同时满足二者之一：（1）原会话身份仍当前；（2）本attempt登记的marker同时匹配本模块ownedPublication和持久化marker。API和Pinia两次发布都登记所有权，未在finally采纳任意当前身份。非交互Passkey/OAuth入口保持原样，不扩大其既有验收范围。

新增auth.storage.http.spec.ts正式33例RED；修复后首轮Storage36与既有205共241例PASS。补充6例pending marker尚未写入即失败、3例不推进交互序号的后续logout，Storage45例PASS，防止通过无条件finally采纳身份来误清后来生命周期。

冻结增量见[reviewed-inputs.json](reviewed-inputs.json)，清单SHA256 `0457d3f26e04900a526dd9ac6498fb0e3615483cb0d5c9a33cc0630705d983c1`。顺序独立规格→质量审查均PASS，见[reviews.json](reviews.json)：规格另加真实HTTP旧异常延迟后setToken保护3/3，质量另加外部存储pending/ready切换保护6/6；后者是调度/存储模拟，不冒称原生跨页时序证明。历史c4报告不代表修复后的资源。

## 新构建证据

`make build-frontend` PASS，356资源完整哈希见[built-resources.json](built-resources.json)。静态入口SHA256 `553b6169477c027e59d4347e6a589de457552eb54f3d4b9fe8be6af4cbb5b8a3`。

- [Storage原生浏览器](browser-fixed-evidence/report.json)：9/9清理及恢复；三个入口API/Pinia auth_user一次写入异常及未登录起点；真实编译动作及回环HTTP，异常包装器留在原位、无手动恢复。JS错误及溢出均0。
- [迟到交互响应](browser-interactive-evidence/report.json)：27/27；先验证目标会话完整采用，再验证旧响应不改变Pinia或持久化状态。
- [答题/盲盒续期](browser-refresh-evidence/report.json)：4/4；主动及401续期，重复HTTP提交仍各仅一次夹具业务提交。
- [原生跨页](browser-cross-tab-evidence/report.json)：8/8，未包装WebLocks；包含真实61秒profile timer观察。
- `make test-frontend`完整退出0：lint/design/typecheck PASS，502文件、3789项PASS、8项SKIP。首次Fork唯一失败为未改后端ImageTaskWorker并发测试的结束时Active计数（1而非0）；单例连续10次及完整失败组复核通过，完整Fork重跑退出0通过。详见[verification.json](verification.json)，保留首次失败日志。

固定前端源码之后按主线程指示复用未变后端的已通过门禁与隔离PG证据，历史命令失败及失败包定向复核见上级[verification.json](../verification.json)；未将旧失败进程改标为成功。当前后端、CI、迁移与主线fdec971d完全一致，本增量只改三份公共auth源码及新Storage测试。新前端全测/构建及Fork另跑；这不构成重新运行整条make test/make build的声明。

## 复现与边界

仓库正式测试：`pnpm --dir frontend exec vitest run src/stores/__tests__/auth.storage.http.spec.ts`。原生浏览器：`python3 docs/archive/completed-tasks/2026-10-interactive-auth/root-storage/browser-storage.py --dist backend/internal/web/dist --out /tmp/storage-browser-evidence --assert-clean`。历史临时probe/config保留原绝对执行路径，可按本机工作树调整；base/head日志固定为修复前证据，不应覆盖。

范围限一次性setItem抛错后存储恢复可用。未覆盖持续不可写、getItem/removeItem故障、完整Passkey/OAuth矩阵或未发布跨页登录意图全局排序。原生浏览器证据不是表单、TOTP、生产JWT、浏览器到PG或生产本机验收；不访问生产、不合并部署。准确新提交与其CI/主线程增量根审见[草稿#362](https://github.com/tqytwe/sub2api/pull/362)，旧c4禁止合并。

后端失败的同步边界：测试只等待processor.completed达到4；worker在Process返回之后才等待heartbeat结束并Ack。源码存在可解释Active短暂非零的窗口，但本批不把它宣称为已测定的根因，也不修改该后端范围。
