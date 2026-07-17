import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PlayOpsView from '@/views/admin/PlayOpsView.vue'

const {
  getSummary,
  getArenaLeaderboard,
  listCampaigns,
  createCampaign,
  listTeams,
  getTeam,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getSummary: vi.fn(),
  getArenaLeaderboard: vi.fn(),
  listCampaigns: vi.fn(),
  createCampaign: vi.fn(),
  listTeams: vi.fn(),
  getTeam: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
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
  const messages: Record<string, string> = {
    'admin.playOps.campaignsTitle': '限时活动',
    'admin.playOps.ruleRechargeBonus': '充值加赠 +{pct}%',
    'admin.playOps.ruleBlindboxExtra': '盲盒每日 +{count} 次',
    'admin.playOps.ruleArenaMultiplier': 'Arena 积分 ×{mult}',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let value = messages[key] || key
        Object.entries(params || {}).forEach(([name, replacement]) => {
          value = value.replace(`{${name}}`, String(replacement))
        })
        return value
      },
    }),
  }
})

function mountView() {
  return mount(PlayOpsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
      },
    },
  })
}

describe('PlayOpsView campaigns', () => {
  beforeEach(() => {
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
    listTeams.mockReset().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    getTeam.mockReset()
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
})
