# 上游同步与发布手册

> 状态：active
> 适用分支：`play/main`
> 生产环境：Zeabur / `https://www.jisudeng.com/`
> 最后核验：2026-07-16

## 分支模型

- `upstream/main`：Wei-Shaw/sub2api，只读上游。
- `origin/play/main`：极速蹬生产分支。
- `sync/upstream-YYYYMMDD`：一次上游同步的审查分支。
- 禁止 rebase 或强推 `play/main`；历史必须保留真实 merge 边界。

## 同步前

```bash
git status --short
git fetch upstream --tags
git fetch origin
git switch play/main
git pull --ff-only origin play/main
UPSTREAM_COMMIT="$(git rev-parse upstream/main)"
git switch -c "sync/upstream-$(date +%Y%m%d)"
```

工作区不干净时先确认每项改动归属，不得清除他人的未提交修改。

## 合并与冲突处理

```bash
git merge --no-ff upstream/main
```

按以下顺序审查：

1. `backend/migrations/` 与 Ent Schema。
2. 设置键、DTO、路由注册和依赖注入。
3. 认证、计费、支付、Play、Image Studio 后端行为。
4. 前端路由、侧栏、功能开关、品牌样式和 i18n。
5. 文档、部署模板和脚本。

处理原则：

- 对照 [Fork 定制登记](./FORK_CUSTOMIZATIONS.md) 逐条判断，不对整文件盲选 `ours` 或 `theirs`。
- 已部署迁移不可改写；同编号不同文件名可以并存。
- 保留 Fork 产品不变量，同时吸收上游安全、协议和兼容性修复。
- 用完整 upstream commit 记录基线，不能只依赖 tag 或 merge 标题。

## 本地非服务验证

禁止启动 `go run serve`、`pnpm dev` 或使用 localhost 做产品验收。允许运行测试和构建：

```bash
./scripts/check-fork-integrity.sh
make test
make build
```

检查完成后更新 `FORK_CUSTOMIZATIONS.md` 顶部 upstream commit 和核验日期，并在同步 PR 记录：

- upstream 起止 commit。
- 冲突文件和处理结论。
- 受影响的 `FORK-*` 条目。
- 测试、构建和线上验收结果。
- 如需回滚，对应 merge commit。

## 合并、部署与生产验收

同步分支 CI 全部通过后，以 merge commit 合入 `play/main`，然后：

```bash
git switch play/main
git pull --ff-only origin play/main
./scripts/push-github-and-deploy.sh play/main
```

等待 Zeabur 构建完成，先确认实际部署 commit 与预期 `origin/play/main`
一致且健康检查通过。随后由用户在本地电脑浏览器访问
`https://www.jisudeng.com/`，至少使用游客、普通用户和管理员三种身份检查：

1. 首页、登录、注册、浅色与深色主题。
2. 普通用户和管理员侧栏。
3. Play Hub、签到、Arena、盲盒、答题和 Agent Team 的开关状态。
4. 图像工作室生成、刷新恢复、预览、下载和删除。
5. 游客与登录用户模型价格、分组绑定和真实调用扣费抽样。
6. 支付入口、测试订单到账和充值 boost。
7. 从 `jisudeng.com` 与 `www.jisudeng.com` 发起 OAuth。

功能同时涉及用户页和管理员页时必须两侧闭环检查；视觉修改检查浅色和
深色主题；余额、奖励、计费、统计、配置或迁移修改按风险补充 API 与
数据库数字对账。服务器浏览器、curl、健康检查或 localhost 不能替代用户
本地电脑浏览器验收。完整准则见
[服务器开发与生产验收流程](./DELIVERY_WORKFLOW.md)。

## 回滚

线上回归时先停止继续合并，然后 revert 引入问题的 merge commit：

```bash
git switch play/main
git pull --ff-only origin play/main
git revert -m 1 <merge-commit>
./scripts/push-github-and-deploy.sh play/main
```

禁止 reset 或强推公共分支。回滚后保留失败原因、影响范围、revert commit 和重新上线的验收结果。
