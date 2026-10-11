# P3-1 账号测试保活审查交接

根已授权来源 #350 最小迁入。功能来源 f8733e06fcec44df17c224854fefdb0e48cd5457、
类型断言测试修正 d6c1365947b265f0c494caf8fc0ec0a453cc3c6c；固定最终来源仍为 v2.10.5。

## 文件分工与适配

- account_test_service.go：仅 TestAccountConnection 增加10秒保活生命周期与断连错误返回。所有 Fork 平台分支保留。
- account_test_sse_keepalive.go：独立互斥writer/context，立即提交SSE注释，事件后继续保活；Header map期间暂停，Write/Flush恢复；停止幂等并等协程退出、恢复原writer/context。
- account_test_sse_keepalive_test.go：上游测试适配本地独立状态；已有取消、事件后保活、语义终止、写失败、释放读流及真实HTTP首包/断连。
- account_test_sse_http_test.go：上游真实HTTP测试适配既有 TestAccountConnection（不是本Fork没有的Pelican入口），假上游/合成凭据。

不改 compact共享helper、account_usage_service.go、#363/#364文件、requestledger或迁移。
API/DTO/配置/数据库/前端没有新字段或入口；已有 EventSource/SSE解析忽略注释，后台解析同样不计为模型文本。

## TDD与审查

RED：真实HTTP四个子场景均失败，成功路径无任何keepalive；两个断连子场景返回nil，错误类型符合预期。
初次GREEN的50ms上游夹具提前结束，使断连断言失真；记录保留，不能把该失败宣称真实取消缺陷。
夹具修正：断连场景只由context取消退出；正常HTTP等待11秒，使用真实10秒节拍，至少2次心跳。
修正后定向GREEN+race通过12.492s。完整门禁见最终验证记录；未完成项不得由静态推断代替。
稳定4文件字节哈希见 [p3-1-source.sha256](./p3-1-source.sha256)，可提前交根审，与本地全量门禁并行。
前端两处AccountTestModal只解析`data: `前缀（既有501/954行），心跳注释不成为内容；没有必要搬上游IQ/Pelican UI测试或不存在的Pelican入口。

规格自审：需求是首包与正文静默保活、事件后不停拍、断连取消、心跳非模型输出、终止语义与生命周期保持。
原生/OpenCodeGo/CN/Gemini/Grok/Antigravity/TypeSafe各测试路由不改动，后续广泛账号测试回归确认。
没有新增后台账号探测，没有真实账号/生产请求。

质量自审：写入和读取writer状态采用同一锁；Header访问暂停背景写；取消关闭context，stop等待done后才恢复writer，sync.Once保护重复stop。
独立helper隔离compact重试/计费语义。HTTP网络/上游body读取通过派生context取消。
测试只在stop后读ResponseRecorder，避免测试本身制造race；client关闭body后的服务/上游退出均设超时。

根独立规格/代码质量审查待补；本文件自审不冒充独立批准。未通过完整门禁前不提交推送。
回滚仅移除服务入口生命周期和新增helper；无DB变更，无数据回滚。

## 根审P2：真实ServerTiming首写重入

50a5808的剥离中间件夹具没有覆盖真实全局ServerTiming。根审确认启用配置且角色为admin时，首次Write的finalize调用c.Writer.Header，重入保活mutex；30秒有界子进程RED堆栈已复现。
候选仅将首次同步beat移到wrapper发布之前；原writer栈的一次性finalize完成后，再发布wrapper和启动心跳协程。不修改ServerTiming开关、权限或Header/Write锁。新增外部service_test包通过公开服务构造器与真实Gin middleware验证，避免复制middleware或引入import cycle。
旧四文件字节证明仅对原审查点成立；当前五文件哈希与增量验证对应。根独立复审仍待补。
