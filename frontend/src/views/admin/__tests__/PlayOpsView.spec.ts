import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import PlayOpsView from '@/views/admin/PlayOpsView.vue'

const {
  getSummary,
  getArenaLeaderboard,
  listCampaigns,
  createCampaign,
  listTeams,
  getTeam,
  listTeamMemberCandidates,
  repairTeamMember,
  listTeamEvents,
  showError,
  showSuccess,
  localeState,
} = vi.hoisted(() => ({
  getSummary: vi.fn(),
  getArenaLeaderboard: vi.fn(),
  listCampaigns: vi.fn(),
  createCampaign: vi.fn(),
  listTeams: vi.fn(),
  getTeam: vi.fn(),
  listTeamMemberCandidates: vi.fn(),
  repairTeamMember: vi.fn(),
  listTeamEvents: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  localeState: { value: 'zh-CN' },
}))

vi.mock('@/api/admin/play', () => ({
  default: {
    getSummary,
    getArenaLeaderboard,
    listCampaigns,
    createCampaign,
    updateCampaign: vi.fn(),
    deleteCampaign: vi.fn(),
    listTeams,
    getTeam,
    listTeamMemberCandidates,
    repairTeamMember,
    listTeamEvents,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, Record<string, string>> = {
    'zh-CN': {
      'admin.playOps.campaignsTitle': '限时活动',
      'admin.playOps.ruleRechargeBonus': '充值加赠 +{pct}%',
      'admin.playOps.ruleBlindboxExtra': '盲盒每日 +{count} 次',
      'admin.playOps.ruleArenaMultiplier': 'Arena 积分 ×{mult}',
      'admin.playOps.addMember': '添加成员',
      'admin.playOps.memberRepair.title': '修复战队成员',
      'admin.playOps.memberRepair.search': '搜索用户',
      'admin.playOps.memberRepair.reason': '修复原因',
      'admin.playOps.memberRepair.review': '核对并继续',
      'admin.playOps.memberRepair.confirm': '确认修复',
      'admin.playOps.memberRepair.confirmTitle': '确认成员修复',
      'admin.playOps.memberRepair.confirmAdd': '确认将“{user}”添加到“{team}”？生效时间：{effectiveAt}。',
      'admin.playOps.memberRepair.confirmMove': '确认将“{user}”移动到“{team}”？生效时间：{effectiveAt}。',
      'admin.playOps.memberRepair.effectiveImmediately': '提交时的上海当前时间',
      'admin.playOps.memberRepair.impactTitle': '本月影响预览',
      'admin.playOps.memberRepair.userSpend': '该用户影响消费',
      'admin.playOps.memberRepair.cancel': '取消',
      'admin.playOps.memberRepair.successAdded': '成员已添加',
      'admin.playOps.memberRepair.loadFailed': '加载候选用户失败',
      'admin.playOps.eventsTitle': '事件时间线',
      'admin.playOps.eventsLoadFailed': '加载战队事件失败',
      'admin.playOps.loadFailed': '加载玩法运营数据失败',
      'admin.playOps.settlementStatus.completed': '已完成',
      'admin.playOps.payoutStatus.paid': '已入账',
      'admin.playOps.events.admin_member_added': '管理员添加成员',
      'admin.playOps.eventReason': '原因：{reason}',
      'admin.playOps.eventEffectiveAt': '生效时间：{time}',
      'admin.playOps.eventTeamTransition': '战队变更：#{source} → #{target}',
      'admin.playOps.eventReasons.admin_moved_last_captain': '管理员移动最后一名队长，来源战队自动归档',
    },
    en: {
      'admin.playOps.campaignsTitle': 'Limited events',
      'admin.playOps.ruleRechargeBonus': 'Recharge bonus +{pct}%',
      'admin.playOps.ruleBlindboxExtra': 'Blind box +{count}/day',
      'admin.playOps.ruleArenaMultiplier': 'Arena score x{mult}',
      'admin.playOps.addMember': 'Add member',
      'admin.playOps.memberRepair.title': 'Repair team member',
      'admin.playOps.memberRepair.search': 'Search user',
      'admin.playOps.memberRepair.reason': 'Repair reason',
      'admin.playOps.memberRepair.review': 'Review and continue',
      'admin.playOps.memberRepair.confirm': 'Confirm repair',
      'admin.playOps.memberRepair.confirmTitle': 'Confirm member repair',
      'admin.playOps.memberRepair.confirmAdd': 'Add “{user}” to “{team}”? Effective time: {effectiveAt}.',
      'admin.playOps.memberRepair.confirmMove': 'Move “{user}” to “{team}”? Effective time: {effectiveAt}.',
      'admin.playOps.memberRepair.effectiveImmediately': 'Current Shanghai time when submitted',
      'admin.playOps.memberRepair.impactTitle': 'Current-month impact preview',
      'admin.playOps.memberRepair.userSpend': 'Affected user spend',
      'admin.playOps.memberRepair.cancel': 'Cancel',
      'admin.playOps.memberRepair.successAdded': 'Member added',
      'admin.playOps.memberRepair.loadFailed': 'Failed to load candidate users',
      'admin.playOps.eventsTitle': 'Event timeline',
      'admin.playOps.eventsLoadFailed': 'Failed to load team events',
      'admin.playOps.loadFailed': 'Failed to load Play Ops data',
      'admin.playOps.settlementStatus.completed': 'Completed',
      'admin.playOps.payoutStatus.paid': 'Credited',
      'admin.playOps.events.admin_member_added': 'Admin added member',
      'admin.playOps.eventReason': 'Reason: {reason}',
      'admin.playOps.eventEffectiveAt': 'Effective: {time}',
      'admin.playOps.eventTeamTransition': 'Team change: #{source} → #{target}',
      'admin.playOps.eventReasons.admin_moved_last_captain': 'An admin moved the last captain and the source team was archived automatically',
    },
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let value = messages[localeState.value]?.[key] || key
        Object.entries(params || {}).forEach(([name, replacement]) => {
          value = value.replace(`{${name}}`, String(replacement))
        })
        return value
      },
      locale: localeState,
    }),
  }
})

function mountView() {
  return mount(PlayOpsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        BaseDialog: {
          props: ['show', 'title'],
          template: '<div v-if="show"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>',
        },
        TotpStepUpDialog: true,
      },
    },
  })
}

function teamRepairTargetDetail() {
  return {
    team: {
      id: 9,
      name: '目标战队',
      invite_code: 'TEAM9',
      members: [],
    },
    settlements: [],
    created_at: '2026-07-01T00:00:00Z',
  }
}

function mockTeamRepairTarget() {
  listTeams.mockResolvedValue({
    items: [{ id: 9, name: '目标战队', member_count: 1 }],
    total: 1,
    page: 1,
    page_size: 20,
  })
  getTeam.mockResolvedValue(teamRepairTargetDetail())
}

function memberCandidate(userID = 42, displayName = '测试成员') {
  return {
    user_id: userID,
    display_name: displayName,
    email: `member-${userID}@example.com`,
    status: 'active',
    is_captain: false,
    blockers: [],
    warnings: [],
    impact: {
      user_spend: '10.00000000',
      target_spend_before: '95.00000000',
      target_spend_after: '105.00000000',
      target_pool_before: '2.85000000',
      target_pool_after: '3.15000000',
    },
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

describe('PlayOpsView campaigns', () => {
  beforeEach(() => {
    localeState.value = 'zh-CN'
    getSummary.mockReset().mockResolvedValue({
      total_teams: 0,
      active_teams: 0,
      month_spend: '0',
      estimated_shared_pool: '0',
      pending_failed_settlements: 0,
      monthly_arena_reward_budget: 0,
      daily_arena_reward_budget: 0,
    })
    getArenaLeaderboard.mockReset().mockResolvedValue({ rewards: [], rows: [] })
    listCampaigns.mockReset().mockResolvedValue([
      {
        id: 7,
        name: '开服福利周',
        start_at: '2026-07-18T00:00:00Z',
        end_at: '2026-07-25T00:00:00Z',
        enabled: true,
        created_at: '2026-07-18T00:00:00Z',
        rules: {
          recharge_bonus_pct: 10,
          blindbox_extra_opens: 2,
          arena_score_multiplier: 2,
        },
      },
    ])
    createCampaign.mockReset().mockResolvedValue({})
    showError.mockReset()
    showSuccess.mockReset()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    listTeams.mockReset().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    getTeam.mockReset()
    listTeamMemberCandidates.mockReset().mockResolvedValue({
      items: [],
      effective_at: '2026-07-20T10:00:00+08:00',
    })
    repairTeamMember.mockReset().mockResolvedValue({ status: 'added', warnings: [] })
    listTeamEvents.mockReset().mockResolvedValue([])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads and renders limited campaigns in Play Ops', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listCampaigns).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('限时活动')
    expect(wrapper.text()).toContain('开服福利周')
    expect(wrapper.text()).toContain('充值加赠 +10%')
    expect(wrapper.text()).toContain('盲盒每日 +2 次')
  })

  it('creates a campaign with structured rules', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="new-campaign"]').trigger('click')
    await wrapper.get('[data-testid="campaign-name"]').setValue('暑期限时活动')
    await wrapper.get('[data-testid="campaign-start"]').setValue('2026-08-01T10:00')
    await wrapper.get('[data-testid="campaign-end"]').setValue('2026-08-08T10:00')
    await wrapper.get('[data-testid="campaign-recharge-bonus"]').setValue('12.5')
    await wrapper.get('[data-testid="campaign-blindbox-extra"]').setValue('3')
    await wrapper.get('[data-testid="campaign-arena-multiplier"]').setValue('1.5')
    expect((wrapper.get('[data-testid="campaign-name"]').element as HTMLInputElement).value).toBe('暑期限时活动')

    await (wrapper.vm as unknown as { submitCampaign: () => Promise<void> }).submitCampaign()
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(createCampaign).toHaveBeenCalledWith(expect.objectContaining({
      name: '暑期限时活动',
      enabled: true,
      rules: expect.objectContaining({
        recharge_bonus_pct: 12.5,
        blindbox_extra_opens: 3,
        arena_score_multiplier: 1.5,
      }),
    }))
  })

  it('does not expose an English backend fallback in the Chinese interface', async () => {
    getSummary.mockRejectedValueOnce({
      reason: 'UNMAPPED_PLAY_OPS_ERROR',
      message: 'backend English failure',
    })

    mountView()
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('加载玩法运营数据失败')
    expect(showError).not.toHaveBeenCalledWith('backend English failure')
  })

  it('searches and adds a team member with localized controls', async () => {
    mockTeamRepairTarget()
    listTeamMemberCandidates.mockResolvedValue({
      items: [memberCandidate()],
      effective_at: '2026-07-20T10:00:00+08:00',
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')
    await wrapper.get('[data-testid="member-candidate-query"]').setValue('member@example.com')
    await wrapper.get('[data-testid="search-member-candidates"]').trigger('click')
    await flushPromises()

    expect(listTeamMemberCandidates).toHaveBeenCalledWith(9, expect.objectContaining({
      q: 'member@example.com',
      operation: 'add',
    }))
    expect(wrapper.text()).toContain('测试成员')
    expect(wrapper.text()).toContain('本月影响预览')
    expect(wrapper.text()).toContain('该用户影响消费')
    await wrapper.get('[data-testid="member-candidate-42"]').trigger('click')
    await wrapper.get('[data-testid="member-repair-reason"]').setValue('修复缺失的战队成员关系')
    await wrapper.get('[data-testid="confirm-member-repair"]').trigger('click')
    await flushPromises()

    expect(repairTeamMember).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('确认成员修复')
    expect(window.confirm).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="execute-member-repair"]').trigger('click')
    await flushPromises()

    expect(repairTeamMember).toHaveBeenCalledWith(9, expect.objectContaining({
      user_id: 42,
      operation: 'add',
      reason: '修复缺失的战队成员关系',
    }))
    expect(repairTeamMember.mock.calls[0]?.[1]).not.toHaveProperty('effective_at')
    expect(showSuccess).toHaveBeenCalledWith('成员已添加')
  })

  it('submits an explicit effective time only when the administrator selected one', async () => {
    mockTeamRepairTarget()
    listTeamMemberCandidates.mockResolvedValue({
      items: [memberCandidate()],
      effective_at: '2026-07-10T08:00:00+08:00',
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')
    await wrapper.get('input[type="datetime-local"]').setValue('2026-07-10T08:00')
    await wrapper.get('[data-testid="member-candidate-query"]').setValue('member@example.com')
    await wrapper.get('[data-testid="search-member-candidates"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="member-candidate-42"]').trigger('click')
    await wrapper.get('[data-testid="member-repair-reason"]').setValue('按工单证据回溯本月成员关系')
    await wrapper.get('[data-testid="confirm-member-repair"]').trigger('click')
    await wrapper.get('[data-testid="execute-member-repair"]').trigger('click')
    await flushPromises()

    expect(repairTeamMember).toHaveBeenCalledWith(9, expect.objectContaining({
      user_id: 42,
      operation: 'add',
      effective_at: '2026-07-10T08:00:00+08:00',
      reason: '按工单证据回溯本月成员关系',
    }))
  })

  it('keeps the preview target fixed when another team detail finishes loading', async () => {
    const nextTeam = deferred<ReturnType<typeof teamRepairTargetDetail>>()
    listTeams.mockResolvedValue({
      items: [
        { id: 9, name: '预检战队', member_count: 1 },
        { id: 10, name: '后来战队', member_count: 1 },
      ],
      total: 2,
      page: 1,
      page_size: 20,
    })
    getTeam.mockImplementation((teamID: number) => {
      if (teamID === 9) {
        return Promise.resolve(teamRepairTargetDetail())
      }
      return nextTeam.promise
    })
    listTeamMemberCandidates.mockResolvedValue({
      items: [memberCandidate()],
      effective_at: '2026-07-20T10:00:00+08:00',
    })

    const wrapper = mountView()
    await flushPromises()
    const changingTeam = (wrapper.vm as unknown as { selectTeam: (id: number) => Promise<void> }).selectTeam(10)
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')
    await wrapper.get('[data-testid="member-candidate-query"]').setValue('member@example.com')
    await wrapper.get('[data-testid="search-member-candidates"]').trigger('click')
    await flushPromises()

    nextTeam.resolve({
      ...teamRepairTargetDetail(),
      team: {
        ...teamRepairTargetDetail().team,
        id: 10,
        name: '后来战队',
      },
    })
    await changingTeam
    await flushPromises()

    await wrapper.get('[data-testid="member-candidate-42"]').trigger('click')
    await wrapper.get('[data-testid="member-repair-reason"]').setValue('固定预检战队避免竞态提交')
    await wrapper.get('[data-testid="confirm-member-repair"]').trigger('click')
    await wrapper.get('[data-testid="execute-member-repair"]').trigger('click')
    await flushPromises()

    expect(listTeamMemberCandidates).toHaveBeenCalledWith(9, expect.any(Object))
    expect(repairTeamMember).toHaveBeenCalledWith(9, expect.objectContaining({
      user_id: 42,
    }))
  })

  it('exposes operation and candidate selection state to assistive technology', async () => {
    mockTeamRepairTarget()
    listTeamMemberCandidates.mockResolvedValue({
      items: [memberCandidate()],
      effective_at: '2026-07-20T10:00:00+08:00',
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')

    const operationButtons = wrapper.findAll('[data-testid^="member-repair-operation-"]')
    expect(operationButtons).toHaveLength(2)
    expect(operationButtons[0].attributes('aria-pressed')).toBe('true')
    expect(operationButtons[1].attributes('aria-pressed')).toBe('false')

    await wrapper.get('[data-testid="member-candidate-query"]').setValue('member@example.com')
    await wrapper.get('[data-testid="search-member-candidates"]').trigger('click')
    await flushPromises()
    const candidate = wrapper.get('[data-testid="member-candidate-42"]')
    expect(candidate.attributes('aria-selected')).toBe('false')
    await candidate.trigger('click')
    expect(candidate.attributes('aria-selected')).toBe('true')
  })

  it('clears stale candidates when a repeated preview request fails', async () => {
    mockTeamRepairTarget()
    listTeamMemberCandidates.mockResolvedValueOnce({
      items: [memberCandidate()],
      effective_at: '2026-07-20T10:00:00+08:00',
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')
    await wrapper.get('[data-testid="member-candidate-query"]').setValue('member@example.com')
    await wrapper.get('[data-testid="search-member-candidates"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('测试成员')

    listTeamMemberCandidates.mockRejectedValueOnce({
      reason: 'UNMAPPED_CP1_ERROR',
      message: 'backend English failure',
    })
    await wrapper.get('[data-testid="search-member-candidates"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('测试成员')
    expect(showError).toHaveBeenCalledWith('加载候选用户失败')
    expect(showError).not.toHaveBeenCalledWith('backend English failure')
  })

  it('ignores an older candidate response that arrives after a newer search', async () => {
    mockTeamRepairTarget()
    const first = deferred<{ items: ReturnType<typeof memberCandidate>[]; effective_at: string }>()
    const second = deferred<{ items: ReturnType<typeof memberCandidate>[]; effective_at: string }>()
    listTeamMemberCandidates.mockImplementation((_teamID, params) => {
      return params.q === 'first@example.com' ? first.promise : second.promise
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')
    const input = wrapper.get('[data-testid="member-candidate-query"]')
    await input.setValue('first@example.com')
    await wrapper.get('[data-testid="search-member-candidates"]').trigger('click')
    await input.setValue('second@example.com')
    await input.trigger('keyup.enter')

    second.resolve({
      items: [memberCandidate(52, '新预检成员')],
      effective_at: '2026-07-20T10:01:00+08:00',
    })
    await flushPromises()
    expect(wrapper.text()).toContain('新预检成员')

    first.resolve({
      items: [memberCandidate(42, '旧预检成员')],
      effective_at: '2026-07-20T10:00:00+08:00',
    })
    await flushPromises()
    expect(wrapper.text()).toContain('新预检成员')
    expect(wrapper.text()).not.toContain('旧预检成员')
  })

  it('refreshes Shanghai month boundaries whenever the repair dialog is reopened', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-20T02:00:00Z'))
    mockTeamRepairTarget()

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')

    let effectiveInput = wrapper.get('input[type="datetime-local"][max]')
    expect(effectiveInput.attributes('min')).toBe('2026-07-01T00:00')
    expect(effectiveInput.attributes('max')).toBe('2026-07-20T10:00')

    const cancelButton = wrapper.findAll('button').find(button => button.text() === '取消')
    expect(cancelButton).toBeTruthy()
    await cancelButton!.trigger('click')
    vi.setSystemTime(new Date('2026-08-02T02:00:00Z'))
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')

    effectiveInput = wrapper.get('input[type="datetime-local"][max]')
    expect(effectiveInput.attributes('min')).toBe('2026-08-01T00:00')
    expect(effectiveInput.attributes('max')).toBe('2026-08-02T10:00')
  })

  it('does not submit when the localized confirmation is canceled', async () => {
    mockTeamRepairTarget()
    listTeamMemberCandidates.mockResolvedValue({
      items: [memberCandidate()],
      effective_at: '2026-07-20T10:00:00+08:00',
    })
    vi.mocked(window.confirm).mockReturnValue(false)

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')
    await wrapper.get('[data-testid="member-candidate-query"]').setValue('member@example.com')
    await wrapper.get('[data-testid="search-member-candidates"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="member-candidate-42"]').trigger('click')
    await wrapper.get('[data-testid="member-repair-reason"]').setValue('修复缺失的战队成员关系')
    await wrapper.get('[data-testid="confirm-member-repair"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="cancel-member-repair-confirm"]').trigger('click')
    await flushPromises()

    expect(window.confirm).not.toHaveBeenCalled()
    expect(repairTeamMember).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('修复战队成员')
  })

  it('does not report a successful repair as failed when the refresh request fails', async () => {
    mockTeamRepairTarget()
    getTeam
      .mockReset()
      .mockResolvedValueOnce(teamRepairTargetDetail())
      .mockRejectedValueOnce({
        reason: 'UNMAPPED_CP1_REFRESH_ERROR',
        message: 'backend English refresh failure',
      })
    listTeamMemberCandidates.mockResolvedValue({
      items: [memberCandidate()],
      effective_at: '2026-07-20T10:00:00+08:00',
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')
    await wrapper.get('[data-testid="member-candidate-query"]').setValue('member@example.com')
    await wrapper.get('[data-testid="search-member-candidates"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="member-candidate-42"]').trigger('click')
    await wrapper.get('[data-testid="member-repair-reason"]').setValue('修复缺失的战队成员关系')
    await wrapper.get('[data-testid="confirm-member-repair"]').trigger('click')
    await wrapper.get('[data-testid="execute-member-repair"]').trigger('click')
    await flushPromises()

    expect(repairTeamMember).toHaveBeenCalledTimes(1)
    expect(showSuccess).toHaveBeenCalledWith('成员已添加')
    expect(showError).toHaveBeenCalledWith('加载玩法运营数据失败')
    expect(showError).not.toHaveBeenCalledWith('admin.playOps.memberRepair.submitFailed')
    expect(showError).not.toHaveBeenCalledWith('backend English refresh failure')
  })

  it('localizes settlement statuses and loads the team event timeline', async () => {
    listTeams.mockResolvedValue({
      items: [{ id: 9, name: '目标战队', member_count: 1 }],
      total: 1,
      page: 1,
      page_size: 20,
    })
    getTeam.mockResolvedValue({
      team: { id: 9, name: '目标战队', invite_code: 'TEAM9', members: [] },
      settlements: [{
        settlement: {
          id: 1,
          period_start: '2026-07-01T00:00:00Z',
          pool_amount: '5',
          status: 'completed',
        },
        allocations: [{
          id: 2,
          user_id: 42,
          reward_amount: '5',
          payout_status: 'paid',
        }],
      }],
      created_at: '2026-07-01T00:00:00Z',
    })
    listTeamEvents.mockResolvedValue([{
      id: 3,
      event_type: 'admin_member_added',
      actor_display_name: '管理员',
      subject_display_name: '测试成员',
      created_at: '2026-07-20T02:00:00Z',
      detail: {
        reason_code: 'admin_moved_last_captain',
        reason: '按工单证据修复成员关系',
        effective_at: '2026-07-10T08:00:00+08:00',
        source_team_id: 8,
        target_team_id: 9,
      },
    }])

    const wrapper = mountView()
    await flushPromises()

    expect(listTeamEvents).toHaveBeenCalledWith(9)
    expect(wrapper.text()).not.toContain('completed')
    expect(wrapper.text()).not.toContain('paid')
    expect(wrapper.text()).not.toContain('admin_member_added')
    expect(wrapper.text()).not.toContain('admin_moved_last_captain')
    expect(wrapper.text()).toContain('管理员移动最后一名队长，来源战队自动归档')
    expect(wrapper.text()).toContain('按工单证据修复成员关系')
    expect(wrapper.text()).toContain('生效时间：')
    expect(wrapper.text()).toContain('战队变更：#8 → #9')
  })

  it('keeps team details available when the event timeline fails to load', async () => {
    mockTeamRepairTarget()
    listTeamEvents.mockRejectedValueOnce({
      reason: 'UNMAPPED_CP1_EVENT_ERROR',
      message: 'backend English event failure',
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('目标战队')
    expect(wrapper.find('[data-testid="add-team-member"]').exists()).toBe(true)
    expect(showError).toHaveBeenCalledWith('加载战队事件失败')
    expect(showError).not.toHaveBeenCalledWith('backend English event failure')
  })

  it('renders the member repair workflow in English when the active locale is English', async () => {
    localeState.value = 'en'
    listTeams.mockResolvedValue({
      items: [{ id: 9, name: 'Target Team', member_count: 1 }],
      total: 1,
      page: 1,
      page_size: 20,
    })
    getTeam.mockResolvedValue({
      team: { id: 9, name: 'Target Team', invite_code: 'TEAM9', members: [] },
      settlements: [],
      created_at: '2026-07-01T00:00:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="add-team-member"]').text()).toContain('Add member')
    await wrapper.get('[data-testid="add-team-member"]').trigger('click')
    expect(wrapper.text()).toContain('Repair team member')
    expect(wrapper.text()).toContain('Search user')
    expect(wrapper.text()).toContain('Repair reason')
    expect(wrapper.text()).toContain('Event timeline')
    expect(wrapper.text()).not.toContain('添加成员')
    expect(wrapper.text()).not.toContain('修复战队成员')
  })
})
