import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AgentTeamView from '@/views/public/AgentTeamView.vue'

const {
  getTeamMeMock,
  getTeamSettlementsMock,
  createTeamMock,
  joinTeamMock,
  leaveTeamMock,
  transferTeamMock,
  removeTeamMemberMock,
  copyToClipboardMock,
} = vi.hoisted(() => ({
  getTeamMeMock: vi.fn(),
  getTeamSettlementsMock: vi.fn(),
  createTeamMock: vi.fn(),
  joinTeamMock: vi.fn(),
  leaveTeamMock: vi.fn(),
  transferTeamMock: vi.fn(),
  removeTeamMemberMock: vi.fn(),
  copyToClipboardMock: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isAuthenticated: true,
    user: { id: 103 },
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
    showError: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: copyToClipboardMock,
  }),
}))

vi.mock('@/api/play', () => ({
  default: {
    getTeamMe: (...args: unknown[]) => getTeamMeMock(...args),
    getTeamSettlements: (...args: unknown[]) => getTeamSettlementsMock(...args),
    createTeam: (...args: unknown[]) => createTeamMock(...args),
    joinTeam: (...args: unknown[]) => joinTeamMock(...args),
    leaveTeam: (...args: unknown[]) => leaveTeamMock(...args),
    transferTeam: (...args: unknown[]) => transferTeamMock(...args),
    removeTeamMember: (...args: unknown[]) => removeTeamMemberMock(...args),
  },
}))

const messages: Record<string, string> = {
  'models.loading': '加载中',
  'play.agentTeam.eyebrow': 'PLAY · AGENT TEAM',
  'play.agentTeam.title': 'Agent Team',
  'play.agentTeam.subtitle': '组队共享收益',
  'play.agentTeam.intro': '小队共享奖励',
  'play.agentTeam.ctaGuest': '注册加入小队',
  'agentTeam.disabled': 'Agent Team 暂未开启',
  'agentTeam.created': '小队已创建',
  'agentTeam.joined': '已加入小队',
  'agentTeam.alreadyJoined': '你已在小队中',
  'agentTeam.notFound': '邀请码无效',
  'agentTeam.failed': '操作失败',
  'agentTeam.linkCopied': '邀请码已复制',
  'agentTeam.contributionsTitle': '本月成员贡献',
  'agentTeam.captainBadge': '队长',
  'agentTeam.memberUsageEmpty': '本月暂无 API 消耗记录',
  'agentTeam.spendStats': '{members} 名成员 · 本月实际消费 ${spend}',
  'agentTeam.reachedTier': '已达 {rate}% 返还档，预计共享奖池 ${pool}',
  'agentTeam.noTier': '本月尚未达到首档共享奖励',
  'agentTeam.nextTier': '再消费 ${amount} 达到 ${threshold} 档位',
  'agentTeam.rewardRule': '按成员实际消费比例分配，次月自动结算，每队每月上限 ${cap}',
  'agentTeam.rewardRuleDetail': '共享奖池按成员实际消费比例分配给成员；队长负责邀请和管理，不独占共享奖池。',
  'agentTeam.teamRecord': '本月团队战绩',
  'agentTeam.nextTierTitle': '下一档',
  'agentTeam.moreToNextTier': '再消费 ${amount}',
  'agentTeam.currentTier': '当前 {rate}% 档',
  'agentTeam.tierReached': '已达成',
  'agentTeam.tierLocked': '未达成',
  'agentTeam.inviteCodeLabel': '小队邀请码',
  'agentTeam.copyInviteCode': '复制邀请码',
  'agentTeam.memberSpend': '${spend} · {pct}%',
  'agentTeam.memberTokens': '信息指标：{tokens} tokens',
  'agentTeam.leave': '离开小队',
  'agentTeam.leaveConfirm': '确认离开当前小队？',
  'agentTeam.transfer': '转让队长',
  'agentTeam.transferConfirm': '确认转让？',
  'agentTeam.remove': '移除成员',
  'agentTeam.removeConfirm': '确认移除？',
  'agentTeam.settlementHistory': '结算历史',
  'agentTeam.noSettlements': '暂无已生成的月度结算',
  'agentTeam.poolStatus': '奖池 ${pool} · {status}',
  'agentTeam.status.pending': '待结算',
  'agentTeam.status.processing': '结算中',
  'agentTeam.status.completed': '已完成',
  'agentTeam.status.partial': '部分完成',
  'agentTeam.status.failed': '结算失败',
  'agentTeam.payout.pending': '待发放',
  'agentTeam.payout.processing': '发放中',
  'agentTeam.payout.paid': '已到账',
  'agentTeam.payout.failed': '发放失败',
}

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
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

function mountView() {
  return mount(AgentTeamView, {
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

describe('AgentTeamView competitive layout', () => {
  beforeEach(() => {
    getTeamMeMock.mockResolvedValue({
      enabled: true,
      team: {
        id: 1,
        name: '星火小队',
        invite_code: 'TEAM2026',
        captain_id: 101,
        member_count: 4,
        token_sum: 992910,
        current_month: '2026-07',
        team_spend: '885.20',
        reached_threshold: '800',
        reward_rate: '0.08',
        next_threshold: '1200',
        estimated_pool: '70.82',
        reward_cap: '88.00',
        reward_tiers: [
          { threshold: '500', rate: '0.05' },
          { threshold: '800', rate: '0.08' },
          { threshold: '1200', rate: '0.10' },
        ],
        members: [
          { user_id: 101, display_name: 'QuoRem', joined_at: '2026-07-01T00:00:00Z', token_sum: 421300, token_pct: 42, spend: '368.20', spend_pct: 42 },
          { user_id: 102, display_name: 'Nina Ops', joined_at: '2026-07-01T00:00:00Z', token_sum: 292880, token_pct: 27, spend: '241.70', spend_pct: 27 },
          { user_id: 103, display_name: '你', joined_at: '2026-07-01T00:00:00Z', token_sum: 188420, token_pct: 20, spend: '178.90', spend_pct: 20 },
          { user_id: 104, display_name: 'Rift Agent', joined_at: '2026-07-01T00:00:00Z', token_sum: 90310, token_pct: 11, spend: '96.40', spend_pct: 11 },
        ],
      },
    })
    getTeamSettlementsMock.mockResolvedValue([
      {
        settlement: {
          id: 7,
          team_id: 1,
          period_start: '2026-07-01T00:00:00Z',
          window_start: '2026-07-01T00:00:00Z',
          window_end: '2026-08-01T00:00:00Z',
          team_spend: '885.20',
          reached_threshold: '800',
          reward_rate: '0.08',
          pool_amount: '70.82',
          cap_amount: '88.00',
          status: 'processing',
        },
        allocations: [
          { id: 1, settlement_id: 7, user_id: 101, contribution: '368.20', ratio: '0.42', reward_amount: '29.74', payout_status: 'processing' },
          { id: 2, settlement_id: 7, user_id: 103, contribution: '178.90', ratio: '0.20', reward_amount: '14.16', payout_status: 'failed' },
        ],
      },
    ])
  })

  it('renders team performance, ranked members, and localized settlement states', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('.agent-team-score-panel').text()).toContain('$70.82')
    expect(wrapper.text()).toContain('队长负责邀请和管理，不独占共享奖池')
    expect(wrapper.findAll('.agent-member-card')).toHaveLength(4)
    expect(wrapper.find('.agent-member-card.tone-gold').text()).toContain('QuoRem')
    expect(wrapper.find('.agent-member-card.current').text()).toContain('你')
    expect(wrapper.text()).toContain('结算中')
    expect(wrapper.text()).toContain('发放中')
    expect(wrapper.text()).toContain('发放失败')
  })
})
