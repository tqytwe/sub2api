# P0-2: Arena 奖励机制说明 - 修改指南

> 生成时间：2026-10-06  
> 任务：在 ArenaView.vue 和 PlayHubView.vue 补充奖励机制说明

## 📋 问题描述

**当前问题**：用户误以为"农场能量"（Arena 代币）能直接兑换余额，实际是按排名结算固定金额。

**目标**：明确告知用户 Arena 奖励机制是排名竞争，不是代币兑换。

---

## 🎯 修改内容

### 1. 添加国际化键值

在 `frontend/src/i18n/locales/zh-CN/dashboard.ts` 的 `play.arena` 对象末尾添加：

```typescript
rewardMechanism: {
  title: '奖励机制说明',
  description: '农场代币仅用于排名竞争，不能直接兑换。最终奖励按榜单排名发放固定金额。',
  monthlyTitle: '月榜奖励档位',
  dailyTitle: '日榜奖励档位',
  rankLabel: '第 {rank} 名',
  amountLabel: '{amount}',
  topRanks: 'Top {count}',
  settlement: '结算时间',
  monthlySettlement: '每月最后一天 23:59',
  dailySettlement: '每天 23:59'
}
```

在 `frontend/src/i18n/locales/en/dashboard.ts` 添加对应英文翻译。

### 2. 修改 ArenaView.vue

在榜单展示区域之前添加奖励机制说明卡片。

### 3. 修改 PlayHubView.vue

补充描述文字，说明"按月榜/日榜排名发放固定奖励"。

---

详细代码和验收标准请参考：docs/玩法福利P0修复实施方案.md
