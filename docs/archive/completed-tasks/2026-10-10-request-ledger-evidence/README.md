# 台账集中修订证据

`local-gates.json` 只保存真实执行的命令、起止时间与退出码。失败记录保留，不将后续修复冒充最初通过。`required-contract-results.json` 汇总最后适用源码上的33项明确执行结果；无跳过替代。源码不变的核心/API/路由测试证据按审查记录复用，其余在a581组合重验。

`main348-subtree-comparison.json` 保存合并前后Git对象：repository差异仅为主线新增集成测试；handler/service必须重验。生产源码、测试和CI的最后文件哈希见`final-source-sha256.json`。

浏览器结果来自生产构建静态资源、隔离PostgreSQL、fake upstream及合成身份，不是生产验收。真实原账务扣费为fixture余额100→98.75、usage7/3、已结算1.25。只读错误注入不冒充上游协议测试。截图在visual-reviews目录。

原始日志保留于执行环境`/tmp/ledger-*.log`，本目录只归档无正文/凭据的摘要。没有生产写入、付费调用、签发凭据、停服务、合并或部署。根线程仍负责外部组合复核、最后SHA CI确认及用户本地生产验收。

`verify.cjs`、`states.cjs`、`final-states.cjs`为本轮实际执行脚本的原样副本，使用测试专用header与无签名的fixture标记，只能配合Go浏览器fixture；不包含有效凭据，不得用于生产鉴权。脚本保留执行环境路径便于核对，其他环境运行需替换路径。
