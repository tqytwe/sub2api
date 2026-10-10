# 上游同步与发布手册

> 状态：active
> 适用分支：`play/main`
> 生产环境：Zeabur / `https://www.jisudeng.com/`
> 最后核验：2026-10-09

## 分支模型

- `upstream-ranxi`：`https://github.com/ranxi2001/sub2api.git`，当前直接功能上游，仅跟进正式发布。
- `refs/remotes/upstream-ranxi/releases/<tag>`：本地只读 release ref，避免把外部 v* 标签推送到 Fork 触发发布。
- `origin/play/main`：极速蹬生产分支。
- `sync/upstream-YYYYMMDD`：一次上游同步的审查分支。
- 禁止 rebase 或强推 `play/main`；历史必须保留真实 merge 边界。

旧版流程直接跟进 Wei-Shaw/sub2api 的历史记录保留，但不再用于未来同步。
Go module/import、LICENSE、作者归属和历史 issue 链接不是更新来源，不做批量替换。
源码来源与生产产物严格分开：检查 ranxi 正式版，选择性迁入源码；仅部署经过
Fork 审查、测试和 CI 的 tqytwe 自研构建。不得把 UpdateService/VersionBadge
中的原版二进制下载地址直接换为 ranxi 发布包。

版本锁定信息：`docs/upstream-migrations/source-lock.json`。`release_commit` 表示
分析目标，不能据此声称整版已迁移；实际批次证据写入版本目录账本。

日常交接只维护版本目录的 [交接快照](./upstream-migrations/v2.10.3/handoff.json)，
字段及弃用规则见 [工程交接入口](./PROJECT_HYGIENE.md)。旧差异清单保留固定比较基线，不作为实时状态。

## 同步前

```bash
git status --short
git ls-remote origin refs/heads/play/main
git fetch origin play/main
gh api repos/ranxi2001/sub2api/releases/latest
# 先核实正式 release/tag/commit 并更新 source-lock.json，再抓取该精确 tag。
# 示例为当前锁定发布；后续版本必须重新核实，不能复制旧 SHA。
git fetch --no-tags upstream-ranxi refs/tags/v2.10.3:refs/remotes/upstream-ranxi/releases/v2.10.3
git worktree add -b sync/ranxi-v2.10.3-<batch> <isolated-path> origin/play/main
# 在隔离 worktree 开始修改前运行（首次更新锁定信息需先审查）。
python3 scripts/check_upstream_release.py --strict
python3 -m unittest discover -s scripts -p test_upstream_release_check.py
```

工作区不干净时先确认每项改动归属，不得清除他人的未提交修改。
同步边界必须核对 release 的 `backend/cmd/server/VERSION`、tag 对象和完整 commit。
不一致即停止，不追随 release 后的分支提交。检查脚本只读，不 fetch、merge、push、
写 VERSION 或发布；`--offline` 仅检查固定本地对象，明确不证明远端当前状态。
`--strict` 拒绝脏目录、共享树和生产分支；普通模式可在实现中重跑差异报告。

## 合并与冲突处理

先生成固定 commit 的差异与迁移冲突报告，按独立批次选择性迁入，不执行自动整树合并。
完整功能来源清单、个性化保留项和验收计划见 `docs/upstream-migrations/v2.10.3/README.md`。

按以下顺序审查：

1. `backend/migrations/` 与 Ent Schema。
2. 设置键、DTO、路由注册和依赖注入。
3. 认证、计费、支付、Play、Image Studio 后端行为。
4. 前端路由、侧栏、功能开关、品牌样式和 i18n。
5. 文档、部署模板和脚本。

处理原则：

- 对照 [Fork 定制登记](./FORK_CUSTOMIZATIONS.md) 逐条判断，不对整文件盲选 `ours` 或 `theirs`。
- 已部署迁移不可改写；对同编号不同文件名也必须审查表结构和顺序，优先使用未占用的新编号，禁止覆盖自研 270/271。
- 保留 Fork 产品不变量，同时吸收上游安全、协议和兼容性修复。
- 用完整 upstream commit 记录基线，不能只依赖 tag 或 merge 标题。

## 本地非服务验证

禁止启动 `go run serve`、`pnpm dev` 或使用 localhost 做产品验收。允许运行测试和构建：

```bash
./scripts/check-fork-integrity.sh
make test
make build
```

检查完成后按实际批次更新 `FORK_CUSTOMIZATIONS.md` 和版本账本，未完整迁入时
不得把分析目标 SHA 写成完整同步成果。在同步 PR 记录：

- upstream 起止 commit。
- 冲突文件和处理结论。
- 受影响的 `FORK-*` 条目。
- 测试、构建和线上验收结果。
- 如需回滚，对应 merge commit。

## 合并、部署与生产验收

同步 PR 的单次 CI 全部通过后，以 merge commit 合入 `play/main`，然后：

```bash
git switch play/main
git pull --ff-only origin play/main
./scripts/push-github-and-deploy.sh play/main
```

生产推送不再重复运行完整测试。直接等待 Zeabur 构建完成，先确认实际部署
commit 与预期 `origin/play/main` 一致且健康检查通过。随后由用户在本地电脑浏览器访问
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
