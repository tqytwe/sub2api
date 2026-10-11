# #362 与已合并 #360 的组合验证

普通merge两个父提交：认证修复 `8fa6f31442d6cc437a189b5d5ee6b7f369b012ce` 与主线 `fb2c9747b58ed5d980a36ea6a2316ead2aa03b7e`（#360，父fdec971d2和6d37d1c0）。未rebase/force，不合并PR或部署。

## 源码保全

[source-proof.json](source-proof.json)记录100份传入文件，其中98份逐字节等主线；两份共享文档保留双方内容。唯一冲突为handoff福利条目说明：该条完整结构等8fa，其余全部feature及顶层元数据等主线。四份冻结Storage输入及client等8fa。后端、CI、脚本全等fb2c主线，SQL278未改，未涉及后续281。

## 本次组合检验

- 真实HTTP及组件组合：293/293、9文件；含26 ledger会话、19 ledger页面、3 ledger API、45 Storage、100交互认证、39 auth HTTP、24 auth store、30 Axios client、7 coordinator。
- 临时交集探针4/4：普通/管理员×API/Pinia发布。挂载ledger并保持翻页在途，真实login在auth_user写入时一次抛错；清除会话/旧详情/私有查询，隔离迟到响应，然后正常login恢复。仅一次Storage异常包装器，真实API/Axios/HTTP/Pinia/Vue；恢复前不移除包装器、不手动恢复状态。复用26项正式harness并通过名称过滤仅跑新增4项，输出中的26 skipped是本次过滤，并非正式26项未运行。
- `make build-frontend`退出0：包括i18n3项、vue-tsc和Vite生产构建。design:check（0视觉改动）、相关文件eslint、handoff校验/8测试及docs校验通过。
- 新构建原生浏览器Storage9/9清理及恢复、迟到交互响应27/27，均绑定入口SHA256 `f8a6040200b1468af3f553202cd2a0ba39461f8c61fbaf758b7154b150733b80`。这是合成回环HTTP的编译auth动作证据，不是ledger原生分页/PG/生产验收。

[356份构建资源哈希](built-resources.json)、[原始压缩日志与规范化哈希](log-hashes.json)、[Storage浏览器报告](browser-storage-evidence/report.json)、[交互浏览器报告](browser-interactive-evidence/report.json)。临时交集测试见[ledger-storage-probe.spec.ts](ledger-storage-probe.spec.ts)及[配置](ledger-storage-probe.config.ts)，保存原执行路径，异地复现需调整。

## 复用及未测边界

按主线程明确指示，不重复未改全套。8fa前端全测3789PASS/8SKIP、Storage完整RED/GREEN及Fork完整复核见[上一批](../root-storage/README.md)。主线后端测试/lint、绑定PG/race、13分页/批量HTTP race、embed与Fork证据见[主线组合归档](../../ledger-detail-pagination/merge-fdec971d/README.md)，本批后端/迁移/CI/脚本逐字节一致，只在本次组合运行上述前端及编译资源验证。并未声称重跑完整make test、后端/PG或Fork；新head完整CI另行核对，不沿用旧8fa CI。

26项ledger的自动续期用例有显式auth.checkAuth()调用，不能单凭该项证明原生timer同步；原生认证矩阵单独列出。没有重新运行完整ledger原生HTTP/PG浏览器矩阵，仍保留其既有证据边界。永久Storage不可写、getItem/removeItem故障、完整Passkey/OAuth和未发布跨页意图全局排序均未扩展验证。

顺序独立组合规格→质量审查均PASS（源码保全、会话语义、日志及资源绑定）；准确新head/CI将记入[PR #362](https://github.com/tqytwe/sub2api/pull/362)，归档不宣称合并部署或生产验收完成。
