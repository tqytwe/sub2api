import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountQualityView from '../AccountQualityView.vue'

const api = vi.hoisted(() => ({
  accountsList: vi.fn(),
  listPlans: vi.fn(),
  listHistory: vi.fn(),
  listPresets: vi.fn(),
  createPreset: vi.fn(),
  listTemplates: vi.fn(),
  createTemplate: vi.fn(),
  updateTemplate: vi.fn(),
  deleteTemplate: vi.fn(),
  listResults: vi.fn(),
  runPlan: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  ownershipPreview: vi.fn(),
  reconcileOwnership: vi.fn(),
  restoreOwnership: vi.fn(),
  deleteOwnedPlan: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: { list: api.accountsList },
    scheduledTests: {
      create: api.create,
      update: api.update,
      ownershipPreview: api.ownershipPreview,
      reconcileOwnership: api.reconcileOwnership,
      restoreOwnership: api.restoreOwnership,
      deleteOwnedPlan: api.deleteOwnedPlan
    }
  }
}))
vi.mock('@/api/admin/accountQuality', () => ({
  listPlans: api.listPlans,
  listHistory: api.listHistory,
  listPresets: api.listPresets,
  createPreset: api.createPreset,
  listTemplates: api.listTemplates,
  createTemplate: api.createTemplate,
  updateTemplate: api.updateTemplate,
  deleteTemplate: api.deleteTemplate,
  listResults: api.listResults,
  runPlan: api.runPlan
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const historyItem = (id: number) => ({
  id, plan_id: 3, account_id: 7, account_name: 'account', model_id: 'model',
  status: 'success', response_text: '', error_message: '', latency_ms: 1,
  started_at: '2026-10-30T00:00:00Z', finished_at: '2026-10-30T00:00:01Z',
  created_at: '2026-10-30T00:00:01Z', passed_count: 1, skipped_count: 0, total_count: 1,
  result_ids: [id], pelican_config: { rule_name: 'rule' }
})

const stubs = {
  AppLayout: { template: '<main><slot /></main>' },
  Icon: true
}

describe('admin account quality operations view', () => {
  beforeEach(() => {
    Object.values(api).forEach((mock) => mock.mockReset())
    api.accountsList.mockResolvedValue({ items: [{ id: 7, name: 'account' }] })
    api.listPlans.mockResolvedValue([])
    api.listHistory.mockResolvedValue({ items: [historyItem(100)], next_cursor: 100 })
    api.listPresets.mockResolvedValue([])
    api.listTemplates.mockResolvedValue([])
  })

  it('loads older global history using the returned cursor and appends it', async () => {
    api.listHistory.mockResolvedValueOnce({ items: [historyItem(100)], next_cursor: 100 })
      .mockResolvedValueOnce({ items: [historyItem(99)], next_cursor: 0 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()

    expect(wrapper.text()).toContain('account #7')
    expect(wrapper.text()).toContain('admin.qualityOps.loadMoreHistory')
    await wrapper.findAll('button').find((button) => button.text() === 'admin.qualityOps.loadMoreHistory')!.trigger('click')
    await flushPromises()

    expect(api.listHistory).toHaveBeenNthCalledWith(2, 100)
    expect(wrapper.text()).toContain('rule')
    expect(wrapper.findAll('article')).toHaveLength(0)
    expect(wrapper.text()).toContain('admin.qualityOps.globalHistory')
    expect(wrapper.findAll('button').some((button) => button.text() === 'admin.qualityOps.loadMoreHistory')).toBe(false)
  })

  it('keeps a visible error and allows retry when global history fails', async () => {
    api.listHistory.mockRejectedValueOnce(new Error('history unavailable'))
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()

    expect(wrapper.text()).toContain('history unavailable')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(api.listHistory).toHaveBeenCalledTimes(2)
  })

  it('creates a Jev judged rule with optional instructions', async () => {
    api.create.mockResolvedValue({ id: 5 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()

    const selects = wrapper.findAll('select')
    await selects[1].setValue(7)
    await selects[2].setValue('jev')
    await wrapper.find('[data-testid="quality-model-id"]').setValue('gpt-5.1')
    await wrapper.find('[data-testid="quality-cron-expression"]').setValue('*/30 * * * *')
    await wrapper.findAll('textarea')[0].setValue('say 21')
    await wrapper.find('[data-testid="quality-expected-answer"]').setValue('21')
    await wrapper.findAll('textarea')[1].setValue('accept equivalent wording')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({
      account_id: 7,
      pelican_config: expect.objectContaining({
        quality: expect.objectContaining({
          expected_answer: '21',
          judge: { kind: 'jev', prompt: 'accept equivalent wording' }
        })
      })
    }))
  })

  it('applies a saved preset to a new rule without creating a plan until the form is saved', async () => {
    api.listPresets.mockResolvedValue([{
      id: 12, name: 'Jev check', model_id: 'gpt-5.1', cron_expression: '0 * * * *', enabled: false, max_results: 50,
      created_at: '', updated_at: '',
      pelican_config: { question_kind: 'text', prompt: 'check the answer', quality: { action: 'disable_scheduling', expected_answer: '42', judge: { kind: 'jev', prompt: 'allow equivalent wording' } } }
    }])
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()

    await wrapper.find('[data-testid="quality-preset-select"]').setValue(12)
    await wrapper.find('[data-testid="quality-apply-preset"]').trigger('click')
    await flushPromises()

    expect((wrapper.findAll('input')[0].element as HTMLInputElement).value).toBe('gpt-5.1')
    expect((wrapper.findAll('input')[1].element as HTMLInputElement).value).toBe('0 * * * *')
    expect((wrapper.findAll('textarea')[0].element as HTMLTextAreaElement).value).toBe('check the answer')
    expect((wrapper.find('[data-testid="quality-max-results"]').element as HTMLInputElement).value).toBe('50')
    expect(wrapper.findAll('textarea')).toHaveLength(2)
    expect(api.create).not.toHaveBeenCalled()
    expect(api.createPreset).not.toHaveBeenCalled()
  })

  it('saves a reusable preset without creating an account plan', async () => {
    api.createPreset.mockResolvedValue({
      id: 3, name: 'Saved check', model_id: 'gpt-5.1', cron_expression: '*/30 * * * *', enabled: true, max_results: 100,
      created_at: '', updated_at: '', pelican_config: { prompt: 'say 21', quality: { action: 'observe_only', expected_answer: '21', judge: { kind: 'expected' } } }
    })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()

    await wrapper.find('[data-testid="quality-model-id"]').setValue('gpt-5.1')
    await wrapper.find('[data-testid="quality-cron-expression"]').setValue('*/30 * * * *')
    await wrapper.find('[data-testid="quality-max-results"]').setValue(35)
    await wrapper.findAll('textarea')[0].setValue('say 21')
    await wrapper.find('[data-testid="quality-expected-answer"]').setValue('21')
    await wrapper.find('[data-testid="quality-toggle-save-preset"]').trigger('click')
    await wrapper.find('[data-testid="quality-preset-name"]').setValue('Saved check')
    await wrapper.find('[data-testid="quality-save-preset"]').trigger('click')
    await flushPromises()

    expect(api.createPreset).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Saved check', model_id: 'gpt-5.1', max_results: 35,
      pelican_config: expect.objectContaining({ prompt: 'say 21', quality: expect.objectContaining({ expected_answer: '21' }) })
    }))
    expect(api.create).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.qualityOps.presetSaved')
  })

  it('creates a group rule with account filters and reports sync counts', async () => {
    api.createTemplate.mockResolvedValue({ template: { id: 4 }, created: 2 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.find('[data-testid="quality-template-new"]').trigger('click')
    const form = wrapper.find('[data-testid="quality-template-form"]')
    await form.findAll('input')[0].setValue('7')
    await form.findAll('input')[1].setValue('oauth')
    await form.findAll('input')[2].setValue('active, disabled')
    await form.findAll('input')[3].setValue('prod')
    await form.findAll('input')[4].setValue('gpt-5.1')
    await form.findAll('textarea')[0].setValue('say 21')
    await form.findAll('input')[7].setValue('21')
    await form.trigger('submit')
    await flushPromises()
    expect(api.createTemplate).toHaveBeenCalledWith(expect.objectContaining({ account_filter: { group: '7', type: 'oauth', search: 'prod', statuses: ['active', 'disabled'] }, model_id: 'gpt-5.1' }))
    expect(wrapper.text()).toContain('admin.qualityOps.templates.created')
  })

  it('keeps or deletes linked plans through separate administrator actions', async () => {
    const template = { id: 8, account_filter: { group: '7' }, model_id: 'gpt-5.1', cron_expression: '*/30 * * * *', enabled: true, max_results: 10, plan_ids: [11], pelican_config: { prompt: 'say 21', quality: { action: 'observe_only', expected_answer: '21', judge: { kind: 'expected' } } }, created_at: '', updated_at: '' }
    api.listTemplates.mockResolvedValue([template])
    api.deleteTemplate.mockResolvedValue({ deleted: 1 })
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    const row = wrapper.find('[data-testid="quality-template-row"]')
    await row.findAll('button')[2].trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="quality-template-row"]').findAll('button')[3].trigger('click')
    await flushPromises()
    expect(api.deleteTemplate).toHaveBeenNthCalledWith(1, 8, false)
    expect(api.deleteTemplate).toHaveBeenNthCalledWith(2, 8, true)
    confirm.mockRestore()
  })
})
