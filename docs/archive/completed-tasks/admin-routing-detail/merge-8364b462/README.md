# #363 普通合入正式 routing 后端后的验证

正式主线为 `8364b462feda15f3bf8f3bb2029314d2193b9dba`，其中 #359 最终提交为 `0ceacb2445f79c3e9ddd1dbe44c89210c8e7ffca`；UI 原冻结点为 `7cc5c016d409aaccec82ec6b222a4c54b620b6e3`。本轮普通 merge，不读取未发布后端分支。共享 handoff 仅保留本任务条目，主线其他条目和全局元数据完全保留。

[源与资源保留证据](source-proof.json)确认全部前端仍是原冻结版本；生产后端、迁移、CI 与 scripts 均为正式主线。新增加的 Go 文件只有[隔离验收夹具](../../../../../backend/internal/requestledger/acceptance/routing_ui_pg_fixture_test.go)。356 个新编译资源与上一轮清单逐字相同。本轮无业务行为修改，无新迁移或历史关联回填。

[运行结果](result.json)和[浏览器断言](browser/browser-evidence.json)来自一次性 loopback PostgreSQL：真实 recorder → 异步 writer → PostgreSQL → 查询/HTTP handler → 编译资源。七个受控样本覆盖六状态和零候选边界，实际最多128事件、局部分页、首个已记录否决和两轮选择均得到验证。真实表重命名使查询返回503；账单、usage、attempt第2页仍可读，恢复后可独立重试。合成输入通过真实结算仓库产生1.25用量记录，余额98.75在只读验收前后不变。它不代表外部模型生成或真实token计量。

权限通过真实JWT签名验证、AdminAuth/JWTAuth和PG用户查询执行：游客401、普通用户403、外租户详情404、非法UUID400、缺失请求404；伪造token和客户端身份头不能提权，数据库当前角色/停用/凭据版本会使旧管理员token失效。密钥由运行时随机生成，仅留内存；测试专用签发端点不验证密码，因此不宣称生产密码登录、MFA、会话绑定或刷新撤销已验。

会话验证使用实际Pinia动作及来自真实HTTP的响应字节：同号重登、换号、管理员转用户、退出、倒序登录、存储发布失败与恢复。关闭详情后通过同一document的Details按钮重开，新200成功后才释放旧真实503，仍保持complete且无告警。传输调度只延迟实际响应，未伪造routing响应体。完整页面零未捕获错误。六状态截图位于[浏览器目录](browser/browser-evidence.json)；停止有限动画后捕获，避免入场过渡画面。

这里仍不证明完整scheduler/provider、WS/媒体/实际Anthropic或Gemini dispatch、生产与用户本机验收。既有完整视觉矩阵、正常续期和原会话回归证据继续保留在上级目录；本轮用源/356资源逐字一致加新1280px真实PG截图连接证据，不称重新执行全部旧浏览器矩阵。英文用户360px既有全局header溢出风险不变。历史27条503及gpt22损失不可由本补丁追溯恢复。

初始准备runner因handoff普通merge冲突停止。run1/2分别修复夹具遗漏ent/runtime初始化、错误假定存在token_version列（实际从email/password_hash指纹导出）；run3修复浏览器glob未匹配查询参数；run4修复角色视图变更后必须先清旧链接再显式打开的测试顺序。run5完整通过；run6额外迟到503检查虽通过，但规格审查要求同document重开；run7按此补强通过；run8只关闭截图动画后重新通过。全部先前结果与日志保存在prior-runs，未将独立fixture PASS冒充浏览器整体PASS。

`execution-runner.py`、`full-gates-runner.py`、`browser.cjs`及`prepared-runner.py`是此次执行脚本快照，保留工作机路径用于追溯。执行器只用一次性容器，并为本次创建的容器加随机标签；清理仅针对该标签。准备runner的16项隔离/清理守卫测试通过。重现时将工作目录/脚本路径改为审查checkout，保持拒绝外部测试DSN/Redis、唯一输出目录、loopback资源及同一container标签清理边界。

[全量门禁](validation.json)与[顺序双审](reviews.json)均已通过；75项必执行契约全部运行成功，前端3869通过/8既有跳过。技术提交与冻结head CI在交付时核对；本记录不授权合并或部署。

首次embed构建因/workspace编译临时目录空间不足失败（ENOSPC），见[失败记录](prior-runs/embed-enospc-gates.json)。改用单独/tmp编译临时目录后embed build/test通过；原make test、tag-unit和75项契约结果保留复用，只续跑失败/未运行的门禁，没有清理共享缓存或改源码。提供给根提前审查的Git树为49e2063bdad369044c40d0b19ff50c5c85bc9337，夹具blob为f9536df160461e1c55d2f8b7822716aaa63e06fc；最终提交时再次验证代码字节一致。
