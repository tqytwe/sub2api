# 独立审查记录

## 顺序 1：规格审查

`/root/p1_spec_review` 首审阻塞：定时 raw Axios 401/403 被拒绝却保持登录；同账号 auth/me 更新 auth_user JSON 使并发 refresh 503 误判会话变更并清除当前用户。原探针及正式 RED 均保留摘要/hash。

复审通过：三生产文件 SHA256 与 reviewed-inputs.json 一致，定向119/119，独立原探针3/3转绿；日志 `/tmp/p4-auth-spec-independent-green.log`。检查没有续期后 checkAuth 补救；硬拒绝只退出当前会话、临时故障保留、旧失败不清新会话。模拟 Web Locks 不作真实跨标签页互斥证据。

## 顺序 2：质量审查

`/root/p1_quality_review` 在规格复审通过后开始，源码质量通过，无阻断项。六个冻结输入一致；核实119项定向回归，并独立真实 HTTP 探针3/3通过（`/tmp/auth-quality-probe/result.log`）：同账号重登后的旧资料200拒绝、同会话其他请求续期不丢弃在途资料、store销毁后在途续期结束不重建定时器。审查确认401响应与重试发出前均检查generation/userID，旧失败不清新会话，通知不含凭据、scope销毁清理订阅，原peer窗口保持。

源码通过结论不替代全量门禁、准确head CI或生产验收。最终门禁证据另行绑定。

## 最新基线组合补审

普通merge纳入23d06337后，规格补审通过：7文件hash匹配，Auth生产3文件仍与首次双审相同。全量发现的旧签到夹具两处改用真实setToken新会话入口，身份/余额/晚响应断言不变，63项相关HTTP通过。相对最新base没有P0、后端、迁移或CI差异；handoff仅变P1的ui.note/risks/next，356/357成果保留。签到原有token变化模拟用例不另算原生续期证据。

质量组合补审通过，无新增阻断：53个上游非共享文件逐字节保留，7份冻结hash匹配，签到fixture hash `9b8cdbfd8bcaed313321ccdbce934436d8ab550079ab1ccc2786963219baeb1a`；旧响应及余额断言未放宽。最终组合门禁仍单独记录，不提前计为通过。

最终测试清理补审仍按规格→质量顺序通过：afterAll增加auth.$dispose()，清理setToken真实入口启动的timer/订阅，原5项断言不变，5/5独立定向日志已核实。签到最终hash `76b6909567b56f69f471869968b3a853b0226974a17557563bd59938c8e20dca`；7输入最终一致，3生产文件不变。

最终资料规格核验通过：准确区分历史失败、组合版本门禁、隔离fixture与生产验收边界，源码/日志/产物hash匹配，53个上游非共享文件保留。tag-unit随后于07:33:59 UTC实际exit0，结果补入verification.json；最终资料质量核验接续通过，无阻塞：7份审查输入、全部日志及产物hash一致，组合门禁通过，历史失败记录完整，未冒称准确head CI或生产验收通过。
