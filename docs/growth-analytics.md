# 增长世界 — 埋点与指标

> 状态：active
> 最后核验：2026-07-15
> 前端实现：`frontend/src/utils/growthAnalytics.ts`（写入 `window.dataLayer`，DEV 模式 `console.debug`）。

---

## 1. North Star 指标

| 指标 | 定义 | 用途 |
|------|------|------|
| **激活率** | 注册 → 7 天内至少 1 次生图或 API 调用 | 工作室拉新效果 |
| **D7 留存** | Play 用户 vs 非 Play 用户第 7 日回访 | 任务 / Hub 留存 |
| **工作室 → 充值** | `image_studio_insufficient_balance` → 完成支付 | ARPU |

**Phase 0 基线**（建议跑 2 周后再设 +25% 激活目标）：
- 当前注册 → 7 日 API 调用率
- 当前 D7 留存
- 可选 5% Holdout：不展示工作室入口作对照

---

## 2. 事件清单

| 事件 | 属性 | 触发位置 |
|------|------|----------|
| `image_studio_workspace_view` | — | 进入图像工作室 |
| `image_studio_intent_select` | `intent_id` | 选择模板所属意图 |
| `image_studio_template_select` | `template_id` | 工作台模板选择 |
| `image_studio_generate_click` | `template_id`, `estimated_cost` | 确认生成按钮 |
| `image_studio_generate_success` | `actual_cost`, `count` | 任务完成 / 轮询成功 |
| `image_studio_generate_fail` | `template_id`, `reason` | 任务或请求失败 |
| `image_studio_result_view` | `job_id`, `count` | 最新结果进入作品区 |
| `image_studio_insufficient_balance` | `balance` | 余额不足拦截 |
| `farm_quest_complete` | `quest_key` | `trackQuestCompleteOnce`（每 quest 每日一次） |
| `farm_daily_tab_view` | — | Arena 日榜 Tab 切换 |
| `play_hub_action_click` | `card_key` | `PlayHubView` 卡片 CTA |

### 已有 Sprint A 事件（Hub / Dashboard）

| 事件 | 说明 |
|------|------|
| `play_hub_view` | 进入玩法中枢 |
| `play_hub_action_click` | Hub 内操作（含 Phase 1 扩展 card_key） |
| `dashboard_recharge_cta_click` | Dashboard 充值 Banner（source: balance_low \| first_recharge \| hub） |
| `arena_gap_view` | Arena 差距展示 |
| `checkin_from_hub` | 从 Hub 跳转签到 |

---

## 3. 本地状态键（非 dataLayer）

| Key | 用途 |
|-----|------|
| `studio_first_win` | 首图仪式是否已展示 |
| `image_studio_last_template` | 回访快捷模式模板 ID |
| `image_studio_auto_cleanup` | 图库 7 天自动清理偏好 |
| `farm_quest_tracked_{key}_{date}` | 任务完成埋点去重 |

---

## 4. 接入 GTM / 自建分析

```javascript
// 页面需预先定义
window.dataLayer = window.dataLayer || []

// growthAnalytics 自动 push：
// { event: 'image_studio_generate_success', actual_cost: 0.08, count: 1, ts: 1720... }
```

**建议下游映射**：
- GA4：`event` → `event_name`，其余 → `event_params`
- 自建：按 `ts` + `user_id`（需 GTM 注入）写入 OLAP

---

## 5. 运营看板（Phase 2）

| 看板 | 维度 |
|------|------|
| 工作室漏斗 | intent → template → generate_click → success |
| 任务完成率 | 按 quest_key / 日 |
| 日榜参与 | `farm_daily_tab_view` UV |
| Hub 效能 | `play_hub_action_click` 按 card_key |

当前产品行为见 [Growth / Play](./GROWTH_PLAY.md) 与 [图像工作室](./IMAGE_STUDIO.md)。历史决策见 [2026-07 增长世界 PRD](./archive/2026-07-growth-world-prd.md)。
