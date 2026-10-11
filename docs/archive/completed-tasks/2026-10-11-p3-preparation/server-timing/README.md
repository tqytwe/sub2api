# 根审P2：账号测试与ServerTiming首次写入死锁

基线：PR #366 `50a580819949887551f6f99ffc02dd3c7785efa1`，当前仍不授权合并。
真实挂载：router.go 全局 `ServerTiming(cfg.Server.EnableServerTiming)`；admin role使首次Write的finalize读取 `c.Writer.Header()`。
旧helper在beat持mutex时已将自己发布为c.Writer，因此回调重入同一非重入锁。默认关闭不等于开启路径兼容。

RED使用真实Gin、`ServerTiming(true)`、admin role、真实管理路径、公开AccountTestService与隔离HTTP上游。
30秒子进程timeout堆栈明确出现 `beat → serverTimingResponseWriter.Write/finalize → ServerTimingHeaderValue → accountTestKeepaliveWriter.Header → Mutex.Lock`。
子进程将回归死锁限制在本测试期限内；不依赖生产不存在的WriteTimeout，不访问生产账号。

候选修复：第一次同步beat在原writer栈仍为c.Writer时完成，随后安装wrapper并启动后台ticker。
因此一次性finalize正常输出Server-Timing；Header/Write互斥、可变header暂停、取消和stop等待逻辑均不变。
已有取消时不写入等测试保留，未关闭功能或降低断言。

新增回归断言：真实Server-Timing头非空、首帧注释、内容事件后仍有10秒心跳、成功终止一次、客户端断连、真实request deadline后两端HTTP handler退出；使用race覆盖并发。
前端、schema、SQL、CI/scripts及全部来源差异JSON/TSV保持原字节；原完整门禁只在未变范围复用。
P3当前五文件哈希见 [source manifest](../../../../upstream-migrations/v2.10.5/p3-1-source.sha256)；原四文件证据固定在[原始manifest](../p3-1-original-source.sha256)。

[增量验证](./validation.json)已完成：组合race23.619s、账号回归21.564s、中间件race1.062s、后端构建和受影响lint（0 issues）通过。[源码复用证明](./source-proof.json)与[日志哈希](./logs.json)可核验。根独立复审和新head CI仍须通过。本记录不是解除禁合的批准。
