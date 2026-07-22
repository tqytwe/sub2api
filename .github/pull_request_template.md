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

核验的 upstream commit：`<full commit>` / 不涉及上游同步

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
