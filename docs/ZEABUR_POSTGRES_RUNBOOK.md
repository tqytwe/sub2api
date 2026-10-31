# Zeabur PostgreSQL 生产只读核验手册

本手册只用于项目交接、发布后核验和故障归因。生产数据库操作默认只读；任何修数、补扣、状态修复、删除或迁移执行都必须另有明确变更授权和回退方案。

## 连接边界

- 使用项目/环境/服务 ID 选择 Zeabur PostgreSQL，凭据只从受控环境变量或人工输入取得，不写入命令、日志、Issue、截图或仓库。
- 只在生产 PostgreSQL service exec 中执行短时 SQL；禁止把生产 DSN 复制到本地开发环境。
- 读取结果必须脱敏：不输出邮箱、用户 ID、IP、request ID、prompt、token、Cookie、完整错误正文或余额明细。

## 只读事务模板

```sql
BEGIN READ ONLY;
SET LOCAL statement_timeout = '7s';
SET LOCAL lock_timeout = '2s';
-- 仅执行 schema、聚合、计数和存在性查询
ROLLBACK;
```

每次查询都要显式 `ROLLBACK`。查询失败也要确认事务已回滚；不要在同一会话继续执行未知状态的语句。

Zeabur `service exec` 的非交互式 `psql` 命令必须显式关闭标准输入，例如在命令末尾追加 `< /dev/null`；否则 CLI 可能持续等待终端输入而没有结果。若必须使用临时端口转发，完成查询后立即执行 `service port-forward --disable`，再用 `service port-forward` 查询并记录 `DISABLED` 状态。凭据只在进程环境变量中使用，不打印变量值。

## 发布后最低核验

1. 记录生产分支、精确部署 SHA、deployment ID、服务状态和健康接口结果。
2. 查询 `schema_migrations` 的数量、最新 revision、候选迁移 filename/checksum；将“生产已应用”和“候选待部署”分开记录。
3. 对新增表、索引、约束和关键外键做 `to_regclass`/目录存在性与脱敏聚合检查，不读取业务正文。
4. 查询应用滚动日志的 ERROR/WARN/FATAL 分类和时间窗口；把数据库审计查询自身的 SQL 错误与应用错误分开。
5. 对账务或 settlement 信号只做分组计数和只读抽样，不能凭 `settlement_pending` 推断已扣款，也不能批量标记 settled 或补扣。
6. 核对未认证、普通用户和管理员的 API 状态码；管理员真实写操作必须由用户本机浏览器完成并保留脱敏结果。

## 证据记录

每条记录包含：UTC 时间、项目/环境/服务标识（不含凭据）、部署 SHA、SQL 目的、只读事务结果、脱敏聚合、查询错误（如有）和下一步。滚动日志或 `RUNNING` 状态不能单独证明部署终态成功；必须同时有精确 SHA、健康检查和数据库/日志复核。

## 禁止事项

- 不执行 `UPDATE`、`DELETE`、`TRUNCATE`、`ALTER`、`DROP`、`INSERT` 或 migration runner。
- 不为了消除历史 unknown、过期状态或 settlement pending 而猜测关联、补扣或清理数据。
- 不把生产日志原文、用户内容或凭据写入仓库。
- 发现迁移缺失、权限放宽、5xx 或账务不一致时停止验收，先调查代码版本、启动迁移日志和部署记录。
