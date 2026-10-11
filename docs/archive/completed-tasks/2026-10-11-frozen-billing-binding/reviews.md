# 冻结计费目标绑定顺序审查

基线 `05b44bec5c2d30ad58d009fbe5247ea363c6ee70`；本轮仅审查冻结 ID 绑定修复及对应测试、CI、兼容说明。撤销前在途结算和精度调查不在本轮生产修改中。

## 1. 规格审查

审查者：独立代理 `/root/p1_spec_review`。先完成规格审查并 PASS，再启动代码质量审查。无剩余新增 P1/P2。

- Prepare/Verify 均须 NULL-safe 精确匹配 subscription/entitlement，不改变 fingerprint。
- live/archive 原 link、原上下文精确重放保留；旧 NULL 不成为通配。缺 link 且携带任一目标 ID 的旧 proof 拒绝建立新归属。
- 两个 ID 均 NULL 的旧路径不增加目标声明；无 ledger context 保留原 no-op。
- 兼容36项、复现4项、既有42项及404条完整迁移证据一致。
- 文档两处表述已收窄：proof snapshot 仅检查键、fingerprint、created_at；Verify 可看到同一事务未提交的 proof，认证与结算一起提交。

规格审查后的测试修改仅有三处 deferred Rollback 清理包装，不改变断言或生产实现；质量审查复核后重新运行36/4项。

## 2. 代码质量审查

审查者：独立代理 `/root/p1_quality_review`。PASS，无剩余新增 P1/P2。

- Prepare 在同一 SQL 原子固定两个 ID；NULL-safe 冲突条件不改写旧绑定。
- 旧 proof 缺 link 的拒绝条件覆盖 live/archive，nil/nil 兼容路径不宣称目标归属；归档搬迁是单 SQL 原子操作。
- Verify 位于原 billing 事务，失败回滚；锁顺序、金额、去重和撤销规则未改。
- CI 三套测试分别创建空数据库，保留 `-race`、`pipefail` 和 stdout/stderr 日志。
- 发现三处测试未处理 Rollback 返回值，已改显式忽略清理返回值并复审。最终兼容36/36和复现4/4 `-race` 再次通过。
- 审查者只读执行 gofmt/diff 检查，并核验运行日志、原始 gzip 和展示文本哈希；没有将独立审查写成第二次 PostgreSQL 执行。

最终审查源码 SHA-256：

| 文件 | SHA-256 |
| --- | --- |
| `backend/internal/requestledger/billing.go` | `a1b76fcc4e6f35c981202e95783bca68bca07481767490d7e7d9a0dde0e8a46f` |
| `subscription_binding_compatibility_integration_test.go` | `90938aa1552ab3ca4fb3ef2d6f94a6d38de2d9184356edaa743720425e487210` |
| `subscription_binding_reproduction_integration_test.go` | `714ca552ff0741266750e56fbe847904f4d9ee80ec064f433d77edf92f8a8296` |

双审通过不替代完整 tests/build/integration/Fork/tagged lint/PR CI；最终状态见 [README](./README.md) 与 [定向运行清单](./targeted-evidence.json)。合并、部署及生产验收归主线程处理。
