# sub2api 极速蹬 Fork 开发指南

> 状态：active
> 最后核验：2026-07-15

## 仓库与分支

| 项目 | 说明 |
|------|------|
| 上游仓库 | `Wei-Shaw/sub2api`，remote 为 `upstream` |
| Fork 仓库 | `tqytwe/sub2api`，remote 为 `origin` |
| 生产分支 | `play/main` |
| 技术栈 | Go + Gin + Ent、Vue 3 + TypeScript + pnpm、PostgreSQL、Redis |

`play/main` 包含极速蹬品牌、Growth / Play、图像工作室、模型目录和计费保护等定制。同步上游前必须阅读 [Fork 定制登记](./docs/FORK_CUSTOMIZATIONS.md) 与 [上游同步手册](./docs/UPSTREAM_SYNC_PLAYBOOK.md)。禁止 rebase 或强推 `play/main`。

## 环境要求

- Go 版本以 `backend/go.mod` 为准。
- 前端只使用 pnpm；修改依赖时同步提交 `frontend/pnpm-lock.yaml`。
- PostgreSQL 和 Redis 的地址、账号与密码由环境变量或未跟踪配置提供，不能写进仓库文档。
- Ent schema 修改后运行 `cd backend && go generate ./ent`，并提交生成文件。

不要混用 npm 与 pnpm 的 `node_modules`。新增 Go interface 方法时必须同步更新全部 mock 和 stub。

## 常用命令

安装前端依赖：

```bash
pnpm --dir frontend install --frozen-lockfile
```

Fork 定制完整性检查：

```bash
./scripts/check-fork-integrity.sh
```

完整测试与构建：

```bash
make test
make build
```

单独执行：

```bash
make -C backend test-unit
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
```

测试和构建可以在本地运行，但不得启动 `go run serve`、`pnpm dev` 或把 localhost 当作产品验收环境。

## CI 与提交要求

`.github/workflows/fork-integrity.yml` 对 `play/main`、同步分支及相关 PR 执行：

- Fork 静态不变量与定向行为测试。
- 后端 unit tests。
- 前端 lint、typecheck、Vitest 和 production build。
- 仓库内 Markdown 链接检查。

CI 失败时不得合并。新增极速蹬特有行为必须在同一 PR：

1. 更新 `docs/FORK_CUSTOMIZATIONS.md`。
2. 增加或更新静态保护、后端测试或前端测试。
3. 在 PR 模板中声明受影响的 `FORK-*` 条目和 upstream commit。

## 数据库迁移

- 已部署 SQL 不可修改；修复必须新增迁移。
- 迁移按完整文件名识别，上游与 Fork 可以出现相同数字前缀。
- 极速蹬自定义迁移清单登记在 `FORK-MIGRATION-009`，上游合并时不得按编号覆盖。
- 修改 Ent schema 后检查 SQL migration、生成代码和运行时迁移三者一致。

## 部署与验收

极速蹬生产由 `origin/play/main` 触发 Zeabur 构建：

```bash
./scripts/push-github-and-deploy.sh play/main
```

脚本会拒绝推送 `main`。唯一生产验收入口为 `https://www.jisudeng.com/`。上游 Docker、Apple Container 等文档只作为通用参考，不代表极速蹬发布流程。

## 文档入口

| 文档 | 用途 |
|------|------|
| [项目文档索引](./docs/README.md) | 当前、参考和历史文档的唯一目录 |
| [Fork 定制登记](./docs/FORK_CUSTOMIZATIONS.md) | 不能被上游覆盖的行为与验证 |
| [上游同步手册](./docs/UPSTREAM_SYNC_PLAYBOOK.md) | 分支、冲突、部署与回滚 |
| [图像工作室](./docs/IMAGE_STUDIO.md) | 当前工作台、接口、隐私和资产行为 |
| [Growth / Play](./docs/GROWTH_PLAY.md) | 当前玩法、设置和路由 |
| [模型与价格](./docs/MODEL_PRICING_CN.md) | 参考价、渠道价和实付价关系 |

## 仓库内 Skill

`skills/sub2api-admin/` 提供管理员 CLI，可管理账号、兑换码、错误规则和 Play 运维。完整命令见 `skills/sub2api-admin/references/admin-cli.md`，凭据只通过环境变量传入。
