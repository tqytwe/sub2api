# PR #414 优先调度空快照验收

状态：限定的管理员空快照场景已验证；完整优先调度功能与全站异常排查仍在原队列。维护责任：本任务提交者。

## 修复与发布证据

[PR #414](https://github.com/tqytwe/sub2api/pull/414) 修复前端误把 snapshot 包装对象当作历史观测的问题。服务端 `history: null` 时，前端返回无快照；有历史时读取嵌套观测，并保留直接快照格式兼容。无历史展示“暂无调度快照。”，不显示缺失时间戳产生的 `Invalid Date`。

- 生产基线：PR #412 / `e1626d760cf92349bb68fee5c6537a66e1dcbdc6`。
- 交付 head：`3c98e00f3251f1af5c284c2d78c055f4f682fa82`；合并 SHA：`01fc395e0306b7af031643e3412c41979afec4be`。
- 定向 API/页面回归 6 项、类型检查和定向 ESLint 通过。PR 的 6 项 CI 全部通过，包含完整前后端测试、前端构建、Fork 保护和安全扫描；2026-11-07 重新查询 GitHub 确认结果。
- 对应 merge SHA 的 [Zeabur check](https://zeabur.com/projects/6a51de14c2881a93656fa4c5/services/6a51ee2df04125ac9a34c28c/deployments/6aeddbbf03a5d91147655d57?envID=6a51de14104975fcb46761c3) 为 success；GitHub deployment `7601839396` 在北京时间 2026-11-06 21:58:06 记录 production success。此前发布后首页及 `/health` 返回 200；本次不把旧健康采样冒充重新请求。

## 本地 Chrome 管理员验收

来源：2026-11-07 从浏览器核验聊天 `01a1b0e6-a74c-7bd1-8b4b-d6d248223010` 回传的记录。服务器侧没有直接读取截图，以下是转述的本机验收证据，部署和 CI 证据另见上段。

在生产 `/admin/priority-scheduling` 的独立临时标签中确认管理员登录态，整页 reload 后点击页面“刷新”，等待按钮恢复可用，再检查 DOM：

- 显示“暂无调度快照。”，没有 `Invalid Date` 或 `unavailable`。
- 调度器继续显示“已停用”。
- 初次加载、reload 后、页面刷新完成后的 error/warn 控制台日志均为 `[]`。该标签只访问本页，本次未观察到该页报错。
- 未保存或修改配置，未触发上游调用；原使用记录标签保留，临时标签已关闭。

截图仅保存在用户电脑：`C:/Users/Administrator/.codex/visualizations/2026/11/06/01a1b0e6-a74c-7bd1-8b4b-d6d248223010/2026-11-07-pr414-priority-verification.jpg`。日期目录不作为验收日期，本次验收日期按回传记录为 2026-11-07；截图未复制入仓库。

## 范围与待办

本次只关闭 PR #414 的管理员无历史快照复验项。没有独立验证游客/普通用户、有历史快照、保存配置、真实调度或惩罚恢复，不表示上游优先调度功能完整等价。

此前多页面核查期间出现的共享 DataTable `null.length` 日志没有路由信息；本页本次无错误不能证明该全站线索已解决，仍需按页面单独复现定位。下一功能仍参考 [v2.19.3 功能队列](../../../upstream-migrations/v2.19.3/feature-queue.md)，不因本次复验扩大代码修改范围。

规格审查：验收只覆盖管理员空快照，未把其他身份、真实调度或共享异常写成通过。文档质量审查：CI、部署、健康历史采样与本机转述分别记录，未包含登录凭据或截图内容。
