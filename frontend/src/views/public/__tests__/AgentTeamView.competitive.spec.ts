import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AgentTeamView from '@/views/public/AgentTeamView.vue'

const {
  getTeamDirectoryMock,
  getTeamPublicLeaderboardMock,
  getTeamLeaderboardMock,
  getTeamMeMock,
  getTeamMyApplicationsMock,
  getTeamCaptainApplicationsMock,
  getTeamSeasonsMock,
  getTeamSeasonMock,
  getTeamRewardShowcaseMock,
  getTeamSettlementsMock,
  applyToTeamMock,
  decideTeamApplicationMock,
  rotateTeamInviteMock,
  setTeamRecruitingMock,
  createTeamMock,
  joinTeamMock,
  leaveTeamMock,
} = vi.hoisted(() => ({
  getTeamDirectoryMock: vi.fn(),
  getTeamPublicLeaderboardMock: vi.fn(),
  getTeamLeaderboardMock: vi.fn(),
  getTeamMeMock: vi.fn(),
  getTeamMyApplicationsMock: vi.fn(),
  getTeamCaptainApplicationsMock: vi.fn(),
  getTeamSeasonsMock: vi.fn(),
  getTeamSeasonMock: vi.fn(),
  getTeamRewardShowcaseMock: vi.fn(),
  getTeamSettlementsMock: vi.fn(),
  applyToTeamMock: vi.fn(),
  decideTeamApplicationMock: vi.fn(),
  rotateTeamInviteMock: vi.fn(),
  setTeamRecruitingMock: vi.fn(),
  createTeamMock: vi.fn(),
  joinTeamMock: vi.fn(),
  leaveTeamMock: vi.fn(),
}))

const authState: {
  isAuthenticated: boolean
  user: { id: number } | null
} = {
  isAuthenticated: false,
  user: null,
}

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
    showError: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() }),
}))

vi.mock('@/api/play', () => ({
  default: {
    getTeamDirectory: (...args: unknown[]) => getTeamDirectoryMock(...args),
    getTeamPublicLeaderboard: (...args: unknown[]) => getTeamPublicLeaderboardMock(...args),
    getTeamLeaderboard: (...args: unknown[]) => getTeamLeaderboardMock(...args),
    getTeamMe: (...args: unknown[]) => getTeamMeMock(...args),
    getTeamMyApplications: (...args: unknown[]) => getTeamMyApplicationsMock(...args),
    getTeamCaptainApplications: (...args: unknown[]) => getTeamCaptainApplicationsMock(...args),
    getTeamSeasons: (...args: unknown[]) => getTeamSeasonsMock(...args),
    getTeamSeason: (...args: unknown[]) => getTeamSeasonMock(...args),
    getTeamRewardShowcase: (...args: unknown[]) => getTeamRewardShowcaseMock(...args),
    getTeamSettlements: (...args: unknown[]) => getTeamSettlementsMock(...args),
    applyToTeam: (...args: unknown[]) => applyToTeamMock(...args),
    decideTeamApplication: (...args: unknown[]) => decideTeamApplicationMock(...args),
    rotateTeamInvite: (...args: unknown[]) => rotateTeamInviteMock(...args),
    setTeamRecruiting: (...args: unknown[]) => setTeamRecruitingMock(...args),
    createTeam: (...args: unknown[]) => createTeamMock(...args),
    joinTeam: (...args: unknown[]) => joinTeamMock(...args),
    leaveTeam: (...args: unknown[]) => leaveTeamMock(...args),
  },
}))

const messages: Record<string, string> = {
  'models.loading': '加载中',
  'play.agentTeam.eyebrow': 'PLAY · TEAM COMPETITION',
  'play.agentTeam.title': '战队竞争',
  'play.agentTeam.subtitle': '组队冲榜，共享奖励',
  'agentTeam.liveLeaderboard': '本月战队排行',
  'agentTeam.teamDirectory': '正在招募的战队',
  'agentTeam.apply': '申请加入',
  'agentTeam.history': '历史结算',
  'agentTeam.viewHistory': '查看已结算战绩',
  'agentTeam.ownTeam': '我的战队',
  'agentTeam.estimatedPool': '预计共享奖池',
  'agentTeam.gapToPrevious': '距上一名',
  'agentTeam.memberCapacity': '{current} / {capacity} 名成员',
  'agentTeam.applicationMessage': '申请说明',
  'agentTeam.submitApplication': '提交申请',
  'agentTeam.applicationSubmitted': '申请已提交',
  'agentTeam.captainControls': '队长管理',
  'agentTeam.pendingApplications': '待处理申请',
  'agentTeam.approve': '通过',
  'agentTeam.reject': '拒绝',
  'agentTeam.rotateInvite': '轮换邀请码',
  'agentTeam.inviteRotated': '邀请码已轮换',
  'agentTeam.noPersonalSpend': '不公开个人消费或个人奖励明细',
  'agentTeam.guestRegister': '注册后申请加入',
  'agentTeam.publicProof': '奖励到账证明',
  'agentTeam.anonymous': '匿名用户',
  'agentTeam.failed': '操作失败，请稍后重试',
  'agentTeam.disabled': '战队竞争暂未开启',
  'agentTeam.noTeams': '暂无可展示战队',
  'agentTeam.noHistory': '暂无已结算赛季',
  'agentTeam.applicationPending': '申请待处理',
  'agentTeam.applicationSla': '队长将在 72 小时内处理',
  'agentTeam.applicationExpires': '申请 7 天后失效',
  'agentTeam.inviteJoin': '使用邀请码加入',
  'agentTeam.createLabel': '创建战队',
  'agentTeam.createButton': '创建',
  'agentTeam.joinButton': '加入',
  'agentTeam.joinPlaceholder': '邀请码',
  'agentTeam.leave': '离开战队',
  'agentTeam.leaveConfirm': '确认离开战队？',
  'agentTeam.left': '已离开战队',
  'agentTeam.recruitingOpen': '接受申请',
  'agentTeam.recruitingClosed': '暂停招募',
  'agentTeam.recruitingUpdated': '招募状态已更新',
}

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale: ref('zh-CN'),
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

const publicDirectory = {
  month: '2026-08',
  rows: [
    { team_id: 11, team_name: '北极星战队', member_count: 20, member_capacity: 30, monthly_spend: '1010.60', estimated_pool: '101.06', accepting_applications: true },
    { team_id: 12, team_name: '云端协作组', member_count: 30, member_capacity: 30, monthly_spend: '750.30', estimated_pool: '75.03', accepting_applications: false },
  ],
}

const publicLeaderboard = {
  month: '2026-08',
  total_teams: 48,
  rows: [
    { rank: 1, team_id: 8, team_name: '远航战队', member_count: 28, monthly_spend: '1164.20', estimated_pool: '116.42', gap_to_previous: '0.00' },
    { rank: 4, team_id: 1, team_name: '星火战队', member_count: 12, monthly_spend: '708.20', estimated_pool: '70.82', gap_to_previous: '42.10' },
  ],
}

function mountView() {
  return mount(AgentTeamView, {
    global: {
      stubs: {
        AuthenticatedPlayShell: { template: '<div><slot /></div>' },
        PublicPageToolbar: true,
        PublicPlayBackLink: true,
        SupportFloatingCard: true,
        PlayUserAvatar: { template: '<span class="avatar-stub" />' },
        Icon: true,
        RouterLink: {
          props: ['to'],
          template: '<a :href="typeof to === \'string\' ? to : to.path"><slot /></a>',
        },
      },
    },
  })
}

describe('AgentTeamView competition experience', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authState.isAuthenticated = false
    authState.user = null
    getTeamDirectoryMock.mockResolvedValue(publicDirectory)
    getTeamPublicLeaderboardMock.mockResolvedValue(publicLeaderboard)
    getTeamSeasonsMock.mockResolvedValue([{ id: 7, month: '2026-07', status: 'settled' }])
    getTeamSeasonMock.mockResolvedValue({
      season: { id: 7, month: '2026-07', status: 'settled' },
      total_teams: 24,
      rows: [{ rank: 1, team_id: 8, team_name: '远航战队', member_count: 28, team_spend: '1280', pool_amount: '128', paid_amount: '128', settlement_status: 'completed' }],
    })
    getTeamRewardShowcaseMock.mockResolvedValue({ winners: [{ settlement_month: '2026-07', team_name: '远航战队', display_name: 'mi***@example.com', amount: 12.8, paid_at: '2026-08-01T00:10:00Z' }] })
    getTeamMeMock.mockResolvedValue({ enabled: true })
    getTeamMyApplicationsMock.mockResolvedValue([])
    getTeamCaptainApplicationsMock.mockResolvedValue([])
    getTeamLeaderboardMock.mockResolvedValue({ ...publicLeaderboard, rows: publicLeaderboard.rows.map(row => ({ ...row, is_mine: row.team_id === 1 })) })
    getTeamSettlementsMock.mockResolvedValue([])
    applyToTeamMock.mockResolvedValue({ id: 21, team_id: 11, status: 'pending', requested_at: '2026-08-01T00:00:00Z', sla_due_at: '2026-08-04T00:00:00Z', expires_at: '2026-08-08T00:00:00Z' })
    decideTeamApplicationMock.mockResolvedValue({ id: 22, team_id: 1, status: 'approved' })
    rotateTeamInviteMock.mockResolvedValue({ invite_code: 'rotated-code', expires_at: '2026-08-31T00:00:00Z' })
    setTeamRecruitingMock.mockResolvedValue({ recruiting: true })
  })

  it('lets guests inspect public competition and routes a team application to registration', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getTeamDirectoryMock).toHaveBeenCalledOnce()
    expect(getTeamPublicLeaderboardMock).toHaveBeenCalledOnce()
    expect(getTeamMeMock).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="team-live-leaderboard"]').text()).toContain('远航战队')
    expect(wrapper.get('[data-testid="team-directory"]').text()).toContain('北极星战队')
    expect(wrapper.get('[data-testid="team-apply-11"]').attributes('href')).toContain('/register')
    expect(wrapper.text()).toContain('不公开个人消费或个人奖励明细')
  })

  it('lets a signed-in user without a team submit a visible, connected application', async () => {
    authState.isAuthenticated = true
    authState.user = { id: 103 }

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="team-apply-11"]').trigger('click')
    await wrapper.get('[data-testid="team-application-message"]').setValue('希望一起冲榜')
    await wrapper.get('[data-testid="team-application-submit"]').trigger('click')
    await flushPromises()

    expect(applyToTeamMock).toHaveBeenCalledWith(11, '希望一起冲榜')
    expect(wrapper.text()).toContain('申请待处理')
    expect(wrapper.text()).toContain('队长将在 72 小时内处理')
    expect(wrapper.html()).not.toContain('applicant_user_id')
  })

  it('highlights the member team, defers historical reads, and never renders member spend details', async () => {
    authState.isAuthenticated = true
    authState.user = { id: 103 }
    getTeamMeMock.mockResolvedValue({
      enabled: true,
      team: {
        id: 1,
        name: '星火战队',
        captain_id: 101,
        member_count: 12,
        estimated_pool: '70.82',
        team_spend: '708.20',
        reward_cap: '300.00',
      },
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="team-own-summary"]').text()).toContain('星火战队')
    expect(wrapper.get('[data-testid="team-leaderboard-row-1"]').classes()).toContain('team-leaderboard-row--mine')
    expect(wrapper.text()).not.toContain('本月成员贡献')
    expect(wrapper.text()).not.toContain('个人预计奖励')
    expect(getTeamSeasonsMock).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="team-history-load"]').trigger('click')
    await flushPromises()

    expect(getTeamSeasonsMock).toHaveBeenCalledOnce()
    expect(getTeamSeasonMock).toHaveBeenCalledWith('2026-07', 10)
    expect(wrapper.get('[data-testid="team-history"]').text()).toContain('远航战队')
  })

  it('keeps the public leaderboard available while member-only supplemental data is slow', async () => {
    authState.isAuthenticated = true
    authState.user = { id: 103 }
    getTeamMeMock.mockResolvedValue({
      enabled: true,
      team: {
        id: 1,
        name: '星火战队',
        captain_id: 101,
        member_count: 12,
        estimated_pool: '70.82',
        team_spend: '708.20',
        reward_cap: '300.00',
      },
    })
    getTeamLeaderboardMock.mockReturnValue(new Promise(() => undefined))
    getTeamSettlementsMock.mockReturnValue(new Promise(() => undefined))

    const wrapper = mountView()
    await flushPromises()

    expect(getTeamLeaderboardMock).toHaveBeenCalledOnce()
    expect(getTeamSettlementsMock).toHaveBeenCalledOnce()
    expect(wrapper.get('[data-testid="team-live-leaderboard"]').text()).toContain('远航战队')
    expect(wrapper.get('[data-testid="team-own-summary"]').text()).toContain('星火战队')
  })

  it('shows the captain-only application queue and invite rotation without exposing applicant IDs', async () => {
    authState.isAuthenticated = true
    authState.user = { id: 101 }
    getTeamMeMock.mockResolvedValue({
      enabled: true,
      team: {
        id: 1,
        name: '星火战队',
        captain_id: 101,
        member_count: 12,
        estimated_pool: '70.82',
        team_spend: '708.20',
        reward_cap: '300.00',
      },
    })
    getTeamCaptainApplicationsMock.mockResolvedValue([
      { id: 22, team_id: 1, applicant_user_id: 9988, status: 'pending', message: '希望加入', requested_at: '2026-08-01T00:00:00Z', sla_due_at: '2026-08-04T00:00:00Z', expires_at: '2026-08-08T00:00:00Z' },
    ])

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="team-application-approve-22"]').trigger('click')
    await wrapper.get('[data-testid="team-invite-rotate"]').trigger('click')
    await flushPromises()

    expect(getTeamCaptainApplicationsMock).toHaveBeenCalledOnce()
    expect(decideTeamApplicationMock).toHaveBeenCalledWith(22, 'approve')
    expect(rotateTeamInviteMock).toHaveBeenCalledOnce()
    expect(wrapper.html()).not.toContain('9988')
  })
})
