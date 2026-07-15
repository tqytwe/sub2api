## 变更说明

简述用户可见行为、风险和回滚方式。

## Fork 定制检查

- [ ] 已确认本 PR 是否修改极速蹬定制。
- [ ] 如有修改，已填写受影响的 `FORK-*` ID，并更新 `docs/FORK_CUSTOMIZATIONS.md`。
- [ ] 如有修改，已新增或更新对应静态检查、后端测试或前端测试。
- [ ] 已检查迁移、计费、路由、品牌、OAuth 和部署影响。
- [ ] 已运行 `./scripts/check-fork-integrity.sh`。
- [ ] 未启动本地产品服务作为验收依据。

受影响的定制 ID：`FORK-...` / 无

核验的 upstream commit：`<full commit>` / 不涉及上游同步

## 验证

列出测试、构建、Zeabur 状态和生产环境验收结果。
