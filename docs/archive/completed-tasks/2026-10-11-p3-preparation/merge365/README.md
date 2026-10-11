# #365组合增量证据

普通merge `8682dba9d6fe827f3766f065f8088732436eff89` 的父提交为已审P3 `4a405620587954840978d1f5035da0c614e4122a` 和主线 `16a11abd29b1f37f3be28820b948f2d8400e4e70`，无冲突。当前[composition.json](../../../../upstream-migrations/v2.10.5/composition.json)记录每个继承文件blob、五个P3文件blob、前端树、281 SHA256、#365头9/9 CI及证据复用指针；检测脚本每次重新核验这些文件字节。旧#363组合快照见[previous-composition.json](./previous-composition.json)。

#365未修改P3账号探测路径或ServerTiming，账号探测不调用本次变化的图片响应/ops标识路径；源码交集为空。复用原全量门禁、P2修复定向race/真实HTTP/中间件/构建/lint和#365 PG/协议/namespace证据，不重复启动全套。本次新增检查针对组合继承完整性和清单遗漏；[validation.json](./validation.json)保留结果及日志hash。最终组合head CI和根最小增量审仍待完成。

当前清单6,279文件、209个release增量、448个SQL（41个来源独有SQL决策）。完整覆盖不等于完整迁移或运行时兼容。正式来源锁保持不变；没有新增/修改SQL，281逐字继承。280仍待#364组合，runner在migrations_runner.go:208排序完整文件名、:229逐文件查schema_migrations，不按最高编号截断；不能因281存在跳过280，也不能把本静态结论当成280运行验证。

根消息明确清单已审PASS，4a405620 SSE P2解除且无新增P1/P2。该结论仅覆盖上述已核字节；本次最终head仍待根核验。没有合并PR、部署或访问生产。
