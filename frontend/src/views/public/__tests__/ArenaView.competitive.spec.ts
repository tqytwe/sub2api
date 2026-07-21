import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ArenaView from '@/views/public/ArenaView.vue'

const {
  getArenaCurrentMock,
  getArenaDailyCurrentMock,
  getArenaLeaderboardMock,
  getArenaDailyLeaderboardMock,
  getArenaDailyRewardSummaryMock,
  getQuestsTodayMock,
} = vi.hoisted(() => ({
  getArenaCurrentMock: vi.fn(),
  getArenaDailyCurrentMock: vi.fn(),
  getArenaLeaderboardMock: vi.fn(),
  getArenaDailyLeaderboardMock: vi.fn(),
  getArenaDailyRewardSummaryMock: vi.fn(),
  getQuestsTodayMock: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isAuthenticated: true,
  }),
}))

vi.mock('@/utils/growthAnalytics', () => ({
  trackGrowthEvent: vi.fn(),
  trackQuestCompleteOnce: vi.fn(),
}))

vi.mock('@/api/play', () => ({
  default: {
    getArenaCurrent: (...args: unknown[]) => getArenaCurrentMock(...args),
    getArenaDailyCurrent: (...args: unknown[]) => getArenaDailyCurrentMock(...args),
    getArenaLeaderboard: (...args: unknown[]) => getArenaLeaderboardMock(...args),
    getArenaDailyLeaderboard: (...args: unknown[]) => getArenaDailyLeaderboardMock(...args),
    getArenaDailyRewardSummary: (...args: unknown[]) => getArenaDailyRewardSummaryMock(...args),
    getQuestsToday: (...args: unknown[]) => getQuestsTodayMock(...args),
  },
}))

const messages: Record<string, string> = {
  'models.loading': '加载中',
  'play.arena.eyebrow': 'PLAY · TOKEN 农场',
  'play.arena.title': 'Token 农场',
  'play.arena.subtitle': '消耗也有回报',
  'play.arena.rulesTitle': '奖励规则',
  'play.arena.ctaGuest': '注册查看排行',
  'play.howItWorks': '玩法说明',
  'arena.disabled': 'Token 农场暂未开启',
  'arena.period': '当前周期：{name}',
  'arena.myStats': '第 {rank} 名 · 本期 {tokens} tokens',
  'arena.gapToPrev': '距上一名还差 {gap} tokens',
  'arena.leaderboard': '排行榜',
  'arena.empty': '暂无排行',
  'arena.tokenValue': '{tokens} 枚代币',
  'arena.rpg.season': '赛季',
  'arena.rpg.level': 'Lv.{level}',
  'arena.rpg.farmer': '耕作者',
  'arena.rpg.energyGap': '距下一级还差 {gap} 能量',
  'arena.rpg.dailyQuests': '每日任务',
  'arena.rpg.go': '去完成',
  'arena.rpg.mainField': '主田',
  'arena.rpg.monthTokens': '本月 {tokens} tokens',
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
  'arena.estimatedReward': '当前名次预计 API 余额奖励：${amount}',
  'arena.competitive.settlementCelebrationTitle': '本期奖励已结算',
  'arena.competitive.settlementCelebrationSubtitle': '奖励已按结算快照到账。',
  'arena.competitive.viewDetails': '查看明细',
  'arena.competitive.rewardZone': '奖励区',
  'arena.competitive.keepClimbing': '继续追榜',
  'arena.competitive.topRange': 'Top 10 发放范围',
  'arena.competitive.noRank': '尚未上榜',
  'arena.competitive.podium': '前三名',
  'arena.competitive.questEnergy': '+{energy} 能量',
  'arena.dailySummary.recentTitle': '最近发放',
  'arena.dailySummary.currentTitle': '当前预估',
  'arena.dailySummary.settledAt': '发放时间：{time}',
  'arena.dailySummary.period': '结算周期：{period}',
  'arena.dailySummary.currentPeriod': '当前周期：{period}',
  'arena.dailySummary.paidToday': '今日已发放',
  'arena.dailySummary.delayed': '非今日发放',
  'arena.dailySummary.winners': '{count} 人获奖',
  'arena.dailySummary.total': '合计 ${amount}',
  'arena.dailySummary.noRecent': '暂无已结算日榜',
  'arena.dailySummary.noEstimate': '当前暂无预估奖励',
  'arena.dailySummary.rowReward': '预计 ${amount}',
  'arena.dailySummary.winnerReward': '到账 ${amount}',
  'arena.dailySummary.rankToken': '#{rank} · {tokens} 枚代币',
  'arena.quests.api_call': 'API 调用',
  'arena.quests.image_generate': '出图 1 张',
}

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
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

function period() {
  return {
    id: 1,
    name: '2026-07 月榜',
    start_at: '2026-07-01T00:00:00Z',
    end_at: '2026-08-01T00:00:00Z',
    status: 'active',
  }
}

function mountView() {
  return mount(ArenaView, {
    global: {
      stubs: {
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
    window.sessionStorage.clear()
    vi.clearAllMocks()
    getArenaCurrentMock.mockResolvedValue({
      enabled: true,
      period: period(),
      token_sum: 653910,
      display_token_sum: 653910,
      rank: 5,
      tokens_to_prev_rank: 37310,
      estimated_reward: 5,
    })
    getArenaDailyCurrentMock.mockResolvedValue({
      enabled: true,
      period: { ...period(), name: '2026-07-17 日榜' },
      token_sum: 12000,
      rank: 3,
    })
    getArenaLeaderboardMock.mockResolvedValue({
      enabled: true,
      period: period(),
      rows: [
        { rank: 1, user_id: 11, display_name: 'Mira Studio', token_sum: 982400 },
        { rank: 2, user_id: 12, display_name: 'North API Lab', token_sum: 876120 },
        { rank: 3, user_id: 13, display_name: '星河工作流', token_sum: 744830 },
        { rank: 4, user_id: 14, display_name: 'Dev Pilot', token_sum: 691220 },
        { rank: 5, user_id: 15, display_name: '你', token_sum: 653910 },
      ],
    })
    getArenaDailyLeaderboardMock.mockResolvedValue({
      enabled: true,
      period: { ...period(), name: '2026-07-17 日榜' },
      rows: [
        { rank: 1, user_id: 21, display_name: 'Daily One', token_sum: 22000 },
        { rank: 2, user_id: 22, display_name: 'Daily Two', token_sum: 18000 },
        { rank: 3, user_id: 23, display_name: '你', token_sum: 12000 },
      ],
    })
    getArenaDailyRewardSummaryMock.mockResolvedValue({
      enabled: true,
      recent: {
        period: { ...period(), id: 31, name: '2026-07-16 日榜', status: 'settled' },
        settled_at: '2026-07-17T00:08:00+08:00',
        paid_today: false,
        winners_count: 2,
        total_amount: 0.7,
        winners: [
          { rank: 1, user_id: 21, display_name: 'Daily One', token_sum: 22000, amount: 0.5 },
          { rank: 2, user_id: 22, display_name: 'Daily Two', token_sum: 18000, amount: 0.2 },
        ],
      },
      current: {
        period: { ...period(), id: 32, name: '2026-07-17 日榜' },
        rows: [
          { rank: 1, user_id: 23, display_name: '你', token_sum: 12000, estimated_reward: 0.5 },
          { rank: 2, user_id: 24, display_name: 'Current Two', token_sum: 9000, estimated_reward: 0.2 },
        ],
      },
    })
    getQuestsTodayMock.mockResolvedValue({
      enabled: true,
      energy: 30,
      level: 2,
      energy_to_next_level: 70,
      server_date: '2026-07-17',
      tasks: [
        { key: 'api_call', completed: true, energy: 20 },
        { key: 'image_generate', completed: false, energy: 30, cta_route: '/image-studio' },
      ],
    })
  })

  it('renders podium, reward rules, current user highlight, and quest cards', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('.arena-podium-card')).toHaveLength(3)
    expect(wrapper.find('.arena-podium-card.tone-gold').text()).toContain('Mira Studio')
    expect(wrapper.text()).toContain('奖励发给排行榜上榜用户')
    expect(wrapper.text()).toContain('每日任务能量用于等级/进度展示，不等同于余额到账')
    expect(wrapper.text()).toContain('当前名次预计 API 余额奖励：$5.00')
    expect(wrapper.text()).toContain('充值/活动倍率只影响展示积分和排名，不直接倍增奖励金额')
    expect(wrapper.find('.arena-rank-row.current').text()).toContain('你')
    expect(wrapper.findAll('.arena-quest-card')).toHaveLength(2)
  })

  it('renders daily recent payout and current estimate without labeling yesterday as today', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.arena-rpg-tab').trigger('click')
    await flushPromises()

    expect(getArenaDailyRewardSummaryMock).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('最近发放')
    expect(wrapper.text()).toContain('当前预估')
    expect(wrapper.text()).toContain('结算周期：2026-07-16 日榜')
    expect(wrapper.text()).toContain('非今日发放')
    expect(wrapper.text()).toContain('2 人获奖')
    expect(wrapper.text()).toContain('合计 $0.70')
    expect(wrapper.text()).toContain('到账 $0.50')
    expect(wrapper.text()).toContain('预计 $0.50')
    expect(wrapper.text()).not.toContain('今日排名')
  })

  it('shows the settlement celebration once per settled period', async () => {
    getArenaCurrentMock.mockResolvedValue({
      enabled: true,
      period: { ...period(), status: 'settled' },
      token_sum: 653910,
      display_token_sum: 653910,
      rank: 5,
      tokens_to_prev_rank: 37310,
      estimated_reward: 5,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('.reward-celebration-overlay').text()).toContain('$5.00')

    await wrapper.get('.reward-close').trigger('click')
    await flushPromises()

    expect(window.sessionStorage.getItem('play-arena-settled:monthly:1')).toBe('1')
    expect(wrapper.find('.reward-celebration-overlay').exists()).toBe(false)

    const again = mountView()
    await flushPromises()

    expect(again.find('.reward-celebration-overlay').exists()).toBe(false)
  })
})
