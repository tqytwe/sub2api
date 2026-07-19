# Growth / Play 当前实现

> 状态：active
> 用户入口：`/play`
> 最后核验：2026-07-19

## 定位

Play 是极速蹬的增长与留存层，围绕 API 使用、充值、任务和邀请形成反馈，不替代余额、支付、订阅或 API 计费系统。Play Hub 聚合当前用户的可用玩法、待办、余额、活动和图像工作室状态。

## 已上线能力

| 能力 | 用户路由 | 主要 API | 主开关 |
|------|----------|----------|--------|
| Play Hub | `/play` | `GET /api/v1/play/hub` | 聚合各子开关 |
| 每日签到与补签 | `/check-in` | `/api/v1/play/checkin/*` | `play_checkin_enabled` |
| 月榜与日榜 Arena | `/arena` | `/api/v1/play/arena/*` | `play_arena_enabled`、`play_daily_arena_enabled` |
| 盲盒 | `/blindbox` | `/api/v1/play/blindbox/*` | `play_blindbox_enabled` |
| 每日答题 | `/quiz-quest` | `/api/v1/play/quiz/*` | `play_quiz_enabled` |
| Agent Team | `/agent-team` | `/api/v1/play/teams/*` | `play_agent_team_enabled` |
| 每日任务 | Play Hub | `GET /api/v1/play/quests/today` | `play_daily_quests_enabled` |
| 限时活动 | Dashboard / Play Hub | `GET /api/v1/play/campaigns/active` | `play_campaigns_enabled` |
| 充值 boost | 充值完成后 | 支付履约内部调用 | `play_recharge_boost_enabled` |
| Team Affiliate | Agent Team | Play service | `play_team_affiliate_enabled` |
| 图像工作室联动 | `/image-studio` | `/api/v1/image-studio/*` | `image_studio_enabled` |
| 公共模型与 Teaser | `/models`、首页 | `/api/v1/public/*` | `public_models_enabled` 等 |

公共 Arena 榜单和盲盒最近记录允许游客查看；签到、开盲盒、答题提交、团队操作、Hub、任务和活动用户状态需要 JWT。运行时设置读取失败时 fail-closed，奖励类默认值只在设置缺失时使用。

## 核心设置

- 签到：`play_checkin_daily_reward`、`play_checkin_makeup_enabled`、`play_checkin_streak_milestones`。
- Arena：`play_arena_settlement_rewards`、`play_daily_arena_top_rewards`。
- 盲盒：`play_blindbox_cost`、`play_blindbox_daily_limit`。
- 答题：`play_quiz_reward_per_correct`、`play_quiz_questions_per_day`。
- 充值 boost：持续小时、签到倍率、额外盲盒次数和 Arena 倍率。
- VIP：`play_vip_tiers`，包含 `tier`、`label`、`min_recharge`、`recharge_bonus_pct`、`color_key` 和权益列表。
- 团队返利：Token 门槛和队长奖励。
- 每日任务：`play_daily_quests`。

设置定义、Admin DTO、公开设置和前端类型必须同步更新。奖励写入 Play ledger；支付订单先完成余额履约，再尝试授予 boost，boost 失败只记录警告，不能回滚充值。

## VIP 与充值加赠口径

VIP 默认保留 V0 作为基础档，正式等级为 V1-V5；默认门槛为 `$0 / $50 / $100 / $200 / $500 / $1000`，充值加赠为 `0 / 2 / 4 / 6 / 8 / 10%`。`recharge_bonus_pct` 在服务端钳制到 `0-10`，`color_key` 统一为 `neutral / emerald / sky / indigo / amber / gold`，前端 Play Hub、模型页和公开文档共用同一套颜色。

对用户的统一解释是：VIP 不改变 API 计费公式，不让订单少付钱，也不改变订阅价格；VIP 只影响余额充值成功后的到账加赠。本单按充值前 VIP 等级计算，充值成功后如果升级，下一笔订单才享受新等级。

充值订单使用支付域创建的 `recharge_snapshot` 固化当时口径，包括支付输入金额、基础到账、当前 VIP、VIP 加赠、活动加赠、最终到账、活动 ID 和充值前累计。Play Hub 和订单详情展示的“为什么到账这么多”必须以快照为准，不能用当前配置反推历史订单。

`users.total_recharged` 只累计支付订单的基础到账金额，不包含 VIP 加赠、活动加赠、签到、兑换码或管理员加款。邀请返利基数也使用基础到账金额，避免 VIP 加赠继续放大返利。退款成功时按订单快照回退基础到账累计，用户余额扣减按最终到账余额处理。

## VIP 盲盒奖池

盲盒按用户当前 VIP 使用专属奖池。V0 使用基础 `season-1-v1`，V1-V5 使用 `season-1-vip-v1` 到 `season-1-vip-v5`，默认成本均为 `$0.50`，预计回报从约 `$0.45` 逐级到约 `$0.495`。VIP 奖池升级只调整奖池概率和 RTP 上限，不保证用户稳赚。

`GET /play/blindbox/status` 返回 `pool`、`current_pool`、`next_pool`、`vip_tier`、`expected_reward`、`next_expected_reward`、`pool_version` 和 `rtp_cap`。`POST /play/blindbox/open` 返回中奖金额、净收益、VIP、预计回报和本次 `pool_version`。开箱审计继续写入 `pool_version`，客服查历史记录时以记录版本解释，不能用当前配置反推。

若运营自定义了非默认 `play_blindbox_pool_json`，服务端不会静默替换该自定义奖池；所有 VIP 档会沿用该自定义池，直到运营显式配置新的 VIP 奖池策略。

## Arena / 农场奖励口径

Token 农场奖励按排行榜结算，不按 VIP 或活动倍率直接放大奖励金额。默认月榜奖励为第 1 名 `$50`、Top 3 `$20`、Top 10 `$5`；默认日榜奖励为第 1 名 `$0.5`、Top 3 `$0.2`、Top 10 `$0.1`。Recharge Boost 和 Campaign 的 Arena 倍率只影响展示积分和排名竞争，最终到账仍按结算时排名命中的固定奖励档。

前端 `/arena` 必须展示预计奖励、排名差距和“活动倍率只影响展示积分和排名，不直接倍增奖励金额”的口径，避免用户把农场能量或展示积分误解成余额到账。

## Agent Team 奖励口径

Agent Team 共享奖励先算团队池，再按成员贡献比例分配：`团队奖池 = 团队月消费 × 当前达标比例`，且不超过月封顶；`个人预计奖励 = 个人消费 / 团队总消费 × 团队奖池`。默认档位为 `$20 → 2%`、`$100 → 3%`、`$500 → 4%`、`$2000 → 5%`，默认封顶 `$250`。

页面展示的是实时预计值，会随本月团队消费和成员贡献变化。最终发放以月结快照为准；分配尾差继续沿用后端规则给最大贡献者，同贡献时给较小 user_id。

## 奖励视觉反馈

盲盒开箱、Arena 已结算奖励和 Agent Team 已到账分成复用 `RewardCelebrationOverlay`。盲盒成功后展示开盒、光束、VIP 颜色彩带和奖励卡；高奖使用 jackpot 变体。Agent Team 当前用户看到 `paid` 分配时展示到账卡，并用 sessionStorage 避免同一分配反复弹出。前端必须遵守 `prefers-reduced-motion`，减少动画但保留奖励卡信息。

## 导航与公开页面

用户侧栏以 `/growth-group` 折叠展示 Hub、签到、Arena、盲盒、答题、Agent Team 和邀请返利，并按功能开关过滤。公开玩法页面允许营销浏览，登录后执行动作。普通用户侧栏不展示渠道监控或“可用渠道”，这些属于管理区能力。

## 后台任务与指标

`PlayGrowthRunner` 负责结算到期的日榜周期，并清理过期图像工作室任务。增长事件和指标口径见 [增长埋点](./growth-analytics.md)。

## 当前 backlog

只保留未实施、能直接排期的短列表：

- 为关键奖励与支付联动补充更完整的失败告警和运营看板。
- 基于真实两周基线确定激活率、D7 留存和工作室到充值的目标值。
- 新增玩法或奖励前先完成滥用模型、成本上限和管理员关闭路径设计。

历史战略和阶段评审见 [2026-07 增长世界 PRD](./archive/2026-07-growth-world-prd.md) 与 [2026-07 Play 路线图](./archive/2026-07-growth-play-roadmap.md)，二者不再代表当前实现。
