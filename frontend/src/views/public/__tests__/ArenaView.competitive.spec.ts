import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ArenaView from '@/views/public/ArenaView.vue'

const { getArenaSeasonOverviewMock, getQuestsTodayMock } = vi.hoisted(() => ({
  getArenaSeasonOverviewMock: vi.fn(),
  getQuestsTodayMock: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: true }),
}))

vi.mock('@/utils/growthAnalytics', () => ({
  trackGrowthEvent: vi.fn(),
  trackQuestCompleteOnce: vi.fn(),
}))

vi.mock('@/api/play', () => ({
  default: {
    getArenaSeasonOverview: (...args: unknown[]) => getArenaSeasonOverviewMock(...args),
    getQuestsToday: (...args: unknown[]) => getQuestsTodayMock(...args),
  },
}))

const messages: Record<string, string> = {
  'models.loading': '加载中',
  'common.retry': '重试',
  'play.arena.eyebrow': 'PLAY · TOKEN 农场',
  'play.arena.title': 'Token 农场',
  'play.arena.subtitle': '消耗也有回报',
  'play.arena.rulesTitle': '奖励规则',
  'play.arena.ctaGuest': '注册查看排行',
  'play.howItWorks': '玩法说明',
  'arena.disabled': 'Token 农场暂未开启',
  'arena.loadFailed': '农场数据暂时无法加载，请重试。',
  'arena.anonymous': '匿名用户',
  'arena.period': '当前周期：{name}',
  'arena.myStats': '第 {rank} 名 · 本期 {tokens} tokens',
  'arena.gapToPrev': '距上一名还差 {gap} tokens',
  'arena.estimatedReward': '当前名次预计 API 余额奖励：${amount}',
  'arena.leaderboard': '排行榜',
  'arena.empty': '暂无排行',
  'arena.tokenValue': '{tokens} 枚代币',
  'arena.currentRewards': '当前奖励档位',
  'arena.rewardTier': '排名区间',
  'arena.noRewardTiers': '当前没有可公开的奖励档位。',
  'arena.historyTitle': '历史结算证明',
  'arena.historyHint': '展开后查看最近已结算赛季的前十和实际发放金额。',
  'arena.loadHistory': '查看历史结算',
  'arena.loadingHistory': '加载历史结算...',
  'arena.historyEmpty': '暂时没有可公开的已结算赛季。',
  'arena.rpg.season': '赛季',
  'arena.rpg.level': 'Lv.{level}',
  'arena.rpg.farmer': '耕作者',
  'arena.rpg.energyGap': '距下一级还差 {gap} 能量',
  'arena.rpg.dailyQuests': '每日任务',
  'arena.rpg.go': '去完成',
  'arena.rpg.tabDaily': '日榜',
  'arena.rpg.tabMonthly': '月榜',
  'arena.rpg.campaignBuff': '活动加成中',
  'arena.competitive.mySeason': '我的赛季状态',
  'arena.competitive.rewardTitle': '奖励怎么发',
  'arena.competitive.rewardRuleRanked': '奖励发给排行榜上榜用户，按有效 API Token 消耗统计。',
  'arena.competitive.rewardRuleSettle': '月榜在周期结束后结算；日榜用于即时反馈与小额活动。',
  'arena.competitive.rewardRuleEnergy': '每日任务能量用于等级/进度展示，不等同于余额到账。',
  'arena.competitive.formulaTitle': '结算口径',
  'arena.competitive.formulaRank': '最终奖励按结算时排名匹配固定金额。',
  'arena.competitive.formulaBoost': '充值/活动倍率只影响展示积分和排名，不直接倍增奖励金额。',
  'arena.competitive.rewardZone': '奖励区',
  'arena.competitive.keepClimbing': '继续追榜',
  'arena.competitive.topRange': 'Top 10 发放范围',
  'arena.competitive.noRank': '尚未上榜',
  'arena.competitive.podium': '前三名',
  'arena.competitive.questDone': '已完成',
  'arena.competitive.questEnergy': '+{energy} 能量',
  'arena.monthlySummary.paid': '已到账',
  'arena.monthlySummary.total': '合计 ${amount}',
  'arena.monthlySummary.winners': '{count} 人获奖',
  'arena.monthlySummary.actualPayout': '实际到账',
  'arena.monthlySummary.winnerReward': '到账 ${amount}',
  'arena.quests.api_call': 'API 调用',
  'arena.quests.image_generate': '出图 1 张',
}

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh-CN' },
      te: (key: string) => key in messages,
      t: (key: string, params?: Record<string, unknown>) => {
        let template = messages[key] ?? key
        for (const [name, value] of Object.entries(params ?? {})) {
          template = template.replaceAll(`{${name}}`, String(value))
        }
        return template
      },
    }),
  }
})

enableAutoUnmount(afterEach)

function period(name: string, id: number) {
  return {
    id,
    name,
    start_at: '2026-08-01T00:00:00+08:00',
    end_at: '2026-09-01T00:00:00+08:00',
    status: 'active',
  }
}

function overview(kind: 'daily' | 'monthly', includeHistory = false) {
  const isDaily = kind === 'daily'
  return {
    enabled: true,
    period: period(isDaily ? '2026-08-01 日榜' : '2026-08 月榜', isDaily ? 10 : 11),
    current: {
      enabled: true,
      period: period(isDaily ? '2026-08-01 日榜' : '2026-08 月榜', isDaily ? 10 : 11),
      token_sum: isDaily ? 12000 : 653910,
      display_token_sum: isDaily ? 12000 : 653910,
      rank: isDaily ? 3 : 5,
      tokens_to_prev_rank: isDaily ? 1000 : 37310,
      estimated_reward: isDaily ? 0.2 : 5,
    },
    rows: [
      { rank: 1, display_name: isDaily ? 'da***@example.com' : 'mi***@example.com', token_sum: isDaily ? 22000 : 982400 },
      { rank: 2, display_name: isDaily ? 'no***@example.com' : 'no***@example.com', token_sum: isDaily ? 18000 : 876120 },
      { rank: 3, anonymous: true, token_sum: isDaily ? 12000 : 744830, is_mine: isDaily },
      { rank: 4, display_name: 'de***@example.com', token_sum: 691220 },
      { rank: 5, anonymous: true, token_sum: 653910, is_mine: !isDaily },
    ],
    reward_tiers: [{ rank_max: 1, amount: isDaily ? 0.5 : 20 }, { rank_max: 10, amount: isDaily ? 0.2 : 5 }],
    history: includeHistory
      ? [{
          period: { ...period(isDaily ? '2026-07-31 日榜' : '2026-07 月榜', isDaily ? 9 : 8), status: 'settled' },
          winners_count: 2,
          total_amount: isDaily ? 0.7 : 25,
          winners: [
            { rank: 1, display_name: 'mi***@example.com', token_sum: 22000, reward_amount: isDaily ? 0.5 : 20, payout_status: 'paid', paid_at: '2026-08-01T00:10:00+08:00' },
            { rank: 2, anonymous: true, token_sum: 18000, reward_amount: isDaily ? 0.2 : 5, payout_status: 'paid', paid_at: '2026-08-01T00:10:00+08:00' },
          ],
        }]
      : [],
  }
}

function mountView() {
  return mount(ArenaView, {
    global: {
      stubs: {
        AuthenticatedPlayShell: { template: '<div><slot /></div>' },
        PublicPageToolbar: true,
        PublicPlayBackLink: true,
        SupportFloatingCard: true,
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('ArenaView competitive layout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getArenaSeasonOverviewMock.mockImplementation((kind: 'daily' | 'monthly', includeHistory = false) => Promise.resolve(overview(kind, includeHistory)))
    getQuestsTodayMock.mockResolvedValue({
      enabled: true,
      energy: 30,
      level: 2,
      energy_to_next_level: 70,
      server_date: '2026-08-01',
      tasks: [{ key: 'api_call', completed: true, energy: 20 }, { key: 'image_generate', completed: false, energy: 30, cta_route: '/ai-creation-space' }],
    })
  })

  it('loads only the selected daily board on first paint and keeps public identities masked', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getArenaSeasonOverviewMock).toHaveBeenCalledTimes(1)
    expect(getArenaSeasonOverviewMock).toHaveBeenCalledWith('daily', false)
    expect(wrapper.find('.arena-rpg-tab.active').text()).toContain('日榜')
    expect(wrapper.findAll('.arena-podium-card')).toHaveLength(3)
    expect(wrapper.find('.arena-podium-card.tone-gold').text()).toContain('da***@example.com')
    expect(wrapper.text()).toContain('当前名次预计 API 余额奖励：$0.20')
    expect(wrapper.text()).toContain('当前奖励档位')
    expect(wrapper.text()).toContain('查看历史结算')
    expect(wrapper.findAll('.arena-quest-card')).toHaveLength(2)
    expect(wrapper.html()).not.toContain('user_id')
  })

  it('loads the monthly board only after switching tabs', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('.arena-rpg-tab')[1].trigger('click')
    await flushPromises()

    expect(getArenaSeasonOverviewMock).toHaveBeenCalledWith('monthly', false)
    expect(wrapper.find('.arena-rpg-tab.active').text()).toContain('月榜')
    expect(wrapper.find('.arena-podium-card.tone-gold').text()).toContain('mi***@example.com')
    expect(wrapper.text()).toContain('当前名次预计 API 余额奖励：$5.00')
  })

  it('defers historical payout proof until the user expands it', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).not.toContain('到账 $0.50')
    await wrapper.get('[data-testid="arena-history-load"]').trigger('click')
    await flushPromises()

    expect(getArenaSeasonOverviewMock).toHaveBeenCalledWith('daily', true)
    expect(wrapper.text()).toContain('2026-07-31 日榜')
    expect(wrapper.text()).toContain('到账 $0.50')
    expect(wrapper.text()).toContain('mi***@example.com')
    expect(wrapper.text()).toContain('匿名用户')
  })
})
