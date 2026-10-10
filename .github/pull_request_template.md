## 变更说明

简述用户可见行为、风险和回滚方式。

## 开发与审查

- [ ] 已在服务器隔离 Git worktree 中开发，未覆盖并行工作。
- [ ] 业务行为已按 TDD 完成，保留能证明需求或回归的测试。
- [ ] 每个任务已依次通过规格审查和代码质量审查，审查问题已复审。
- [ ] 未把密码、API key、Cookie、生产凭据或未脱敏日志写入变更。

## Fork 定制检查

- [ ] 已确认本 PR 是否修改极速蹬定制。
- [ ] 如有修改，已填写受影响的 `FORK-*` ID，并更新 `docs/FORK_CUSTOMIZATIONS.md`。
- [ ] 如有修改，已新增或更新对应静态检查、后端测试或前端测试。
- [ ] 已检查迁移、计费、路由、品牌、OAuth 和部署影响。
- [ ] 已运行 `./scripts/check-fork-integrity.sh`。
- [ ] 已运行适用的定向测试、`make test` 和 `make build`，全部通过后才提交/推送。
- [ ] 本 PR 是先推送的审查分支；确认后将以非 rebase、非强推方式进入 `play/main`。
- [ ] 完整 GitHub CI 仅由本 PR 执行一次；普通分支 push 和生产 push 不重复运行完整测试。

受影响的定制 ID：`FORK-...` / 无

直接功能上游：`ranxi2001/sub2api` / 不涉及上游同步

正式 release tag 与完整 commit：`<tag>` / `<full commit>`

本批迁入范围与账本：`docs/upstream-migrations/...`；未迁入项：

交接记录 ID / 核查时间（规则见 `docs/PROJECT_HYGIENE.md`）：

模块/文件归属、负责人或待分配、并行冲突协调：

契约变化（UI/API/默认值/权限/数据/旧调用方兼容；无则说明）：

增量迁移编号/完整文件名、兼容与回退触发条件（不改历史 SQL）：

UI/API/schema/test 关联、真实持久化及用户闭环剩余项：

弃用/移除决策 ID、版本、替代路径、兼容监控、明确批准与保留回归（不涉及则说明）：

- [ ] 已更新同一交接记录的 PR/SHA、证据/风险/待办；后端能力、CI、部署和用户验收未混写。
- [ ] base 变化时使用真实普通 merge 解决共享文件冲突，记录最终 SHA 并复审/重跑受影响门禁，未 force/rebase。

- [ ] 上游源码与 tqytwe 生产构建产物已区分，未向 Fork 推送外部发布标签或用原版二进制覆盖 Fork。
- [ ] 已检查 release 来源/漂移标签、迁移完整文件名/编号/表结构冲突，未改写已部署 SQL。

## 前端视觉检查

- [ ] 本 PR 不包含可见界面改动，或已先阅读 `frontend/AGENTS.md` 与
      `docs/FRONTEND_DESIGN_SYSTEM.md`。
- [ ] 已查看目标页面当前实际画面、至少一个同类型页面和相关共享组件。
- [ ] 若包含可见改动，已新增结构化 `docs/visual-reviews/YYYY-MM-DD-<slug>.md`
      并提交真实的修改前后画面产物。
- [ ] 已检查适用的 hover、active、focus、loading、disabled、empty、error 和 success。
- [ ] 已检查适用的移动/桌面、浅色/深色、中英文、键盘和 reduced-motion。
- [ ] 未新增平行页面框架、功能图标、按钮、表单、浮层或公告渲染体系。

## 自动化验证

Design governance：

定向测试：

完整测试：

完整构建：

Fork integrity：

## 合并后部署与生产验收记录

本节在 PR 合入 `play/main` 后补充，不作为合并前勾选项。任一适用项未通过时，
交付仍保持未完成状态并继续修复。

交付 commit：

Zeabur 部署 commit 与健康状态：

本地浏览器游客验收：

本地浏览器普通用户验收：

本地浏览器管理员验收：

用户/管理员页面闭环：

浅色/深色及 API/数据库对账（如适用）：

未完成、失败或等待项：
