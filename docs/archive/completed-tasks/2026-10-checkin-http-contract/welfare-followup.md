# 玩法 / 福利后续闭环验收清单

状态：后续分批计划；本次只完成普通签到成功响应修复。以 `AppSidebar.vue` 的 `buildGrowthNavChildren` 和 `router/index.ts` 的真实入口为依据，不按名称假设存在独立页面。

| 真实入口及页面 | API / 服务 / 数据 | 后续必须验证的闭环 |
| --- | --- | --- |
| `/check-in` → user/CheckInView | `/play/checkin/status`、POST checkin/makeup；play_service、play_streak；play_checkins、资格快照、能量/奖励/余额流水 | 普通签到错误对象适配；补签前提、昨日/跨天时区、不可重复补签、连续天数/里程碑、现金/券/兑换码投影；余额数值和能量单位分离；多标签页竞争后刷新 |
| `/play` → user/PlayHubView | `/play/hub`、`/play/quests/today`、`/play/campaigns/active`；play_hub、play_quests；play_quest_progress、play_campaigns | 聚合卡片与详细页面开关、资格、已完成状态、实际奖励种类一致；无任务/部分接口失败/加载/刷新；VIP、活动展示与真实配置对应。未见独立能量钱包/任务领取页，禁止假设存在兑换能力 |
| `/quiz-quest` → public/QuizQuestView | quiz/today、quiz/submit；play_extended；play_quiz_questions、play_quiz_attempts、资格/奖励流水 | 游客→登录；题目时间范围、答案合法性、当天重复与并发；计分、能量、现金、券/兑换码显示；已提交后刷新恢复结果；错误/空题库/中文 |
| `/blindbox` → public/BlindboxView | blindbox/status、pool、open、recent；play_extended、play_coupon_rewards；play_blindbox_opens、coupon pools、资格/奖励/余额流水 | 游客和 Explorer 奖池信息边界；已发布/耗尽/未发布；一次开箱扣费与发奖同事务；同 idempotency key 重放及多标签页；日限、时间切换、余额不足；优惠券/兑换码有效期和中奖记录；错误归一化 |
| `/arena` → public/ArenaView | arena/overview、current、leaderboard、daily/current、daily/leaderboard、reward-summary、daily/reward-summary；play_arena_season、play_arena_settle；play_arena_periods、奖励流水 | 日/月/历史时间区间、真实 tokens 与展示积分/倍率区分、排名 ties、未参与/结算中/已结算、预算与分配总额、重复结算无重复奖励、刷新后排名和历史持久化；任务状态联动 |
| `/agent-team` → public/AgentTeamView | teams/me、directory、public/private leaderboard、seasons、applications、join/leave/transfer/remove、invite/rotate、settlements；play team/admission/settlement；play_teams、members、join_applications/events、seasons/rankings、reward_allocations | 游客公开字段、队长/队员权限；申请并发/满员/退出/换队/邀请轮换与过期；赛季边界、贡献/分奖/队长返利不重复；结算历史、空列表、错误与刷新 |
| `/affiliate` → user/AffiliateView 及其引用组件 | `/user/aff`、`/user/aff/transfer`、referral campaign API；affiliate_service、referral_campaign；user_affiliates、user_affiliate_ledger、referral campaign/enrollment/qualification/reward 表 | 邀请归因唯一性、净支付/真实调用资格、风控冻结与解冻、报名/邀请 token 过期和版本、领取并发/幂等、返利转余额对账；CNY/余额单位；活动开始/结束/结算/领取截止；重载恢复已领/冻结/可领取 |
| `/admin/play-ops`、管理端邀请记录/返利/转账及活动管理 | admin play / affiliates endpoints、相关配置与 immutable audit | 用户页和管理员页同一开关/奖池版本/活动时间/资格；历史审批仅历史查看，不复活停用策略；审计关联和奖励汇总匹配；普通用户不能访问管理接口 |
| 券包与福利资格（关联面） | `/coupons/me`、coupon_service、统一 growth eligibility；user_coupons、coupon pools、auth_identities/usage_logs/payment_orders/subscriptions | available/locked/consumed/expired/voided 的展示与持久化；资格邮箱/三天/七天真实调用/三十天净充值/有效订阅边界；券与现金/能量不得混称，不改支付套餐任务文件 |

统一验收步骤：为每个状态机列出初态→动作→终态与失败回滚，使用全量最新迁移的隔离数据库造数据；真实 HTTP 执行后对账记录、唯一键、流水、金额与单位，再让现有 API 客户端驱动页面，重新挂载验证持久化。包含顺序重试与并发、过期、区间左右边界、禁止资格、无数据、加载和错误状态。既存单层 mock 只能辅助。

显示优化在既有 `growth-world.css`、Play/Workspace 布局、Icon、按钮/表格/Toast 中进行。每批先读取前端设计规范及机器清单，记录真实基线和原型，按 360/768/1280/宽屏、浅深色、中英、焦点/禁用/错误/成功完成适用视觉证据；无需修改的页面不强行改风格。

首个已证实的后续缺陷：签到页业务错误读取路径与真实客户端错误对象不一致。其他条目当前仅完成导航/路由/代码索引盘点，未宣称逐项无缺陷。所有模拟账号、金额、支付/用量记录仅存在于隔离数据库；不发起真实付费调用、不向生产发奖或改余额。
