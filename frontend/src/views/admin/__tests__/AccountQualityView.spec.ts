import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountQualityView from '../AccountQualityView.vue'

const api = vi.hoisted(() => ({
  accountsList: vi.fn(),
  groupsAll: vi.fn(),
  removeOwnedModel: vi.fn(),
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
    groups: { getAllIncludingInactive: api.groupsAll },
    scheduledTests: {
      create: api.create,
      update: api.update,
      ownershipPreview: api.ownershipPreview,
      reconcileOwnership: api.reconcileOwnership,
      restoreOwnership: api.restoreOwnership,
      deleteOwnedPlan: api.deleteOwnedPlan,
      removeOwnedModel: api.removeOwnedModel
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
    api.groupsAll.mockResolvedValue([{ id: 10, name: "quarantine" }])
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
          auto_restore: false,
          judge: { kind: 'jev', prompt: 'accept equivalent wording' }
        })
      })
    }))
  })

  it('requires an explicit automatic restore choice and sends it on create', async () => {
    api.create.mockResolvedValue({ id: 5 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    const control = wrapper.find('[data-testid="quality-auto-restore"]')
    expect(control.attributes('aria-checked')).toBe('false')
    await control.trigger('click')
    await wrapper.findAll('select')[1].setValue(7)
    await wrapper.find('[data-testid="quality-model-id"]').setValue('primary')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({
      pelican_config: expect.objectContaining({ quality: expect.objectContaining({ auto_restore: true }) })
    }))
    expect(wrapper.find('[data-testid="quality-auto-restore"]').attributes('aria-checked')).toBe('false')
  })

  it('reads and preserves an existing rule automatic restore choice on edit', async () => {
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    api.listPlans.mockResolvedValue([{
      id: 9, account_id: 7, model_id: 'm', cron_expression: '*/30 * * * *', enabled: true, max_results: 10,
      updated_at: '2026-10-31T00:00:00Z', pelican_config: { prompt: 'say 21', quality: { action: 'disable_scheduling', expected_answer: '21', auto_restore: true, judge: { kind: 'expected' } } }
    }])
    api.update.mockResolvedValue({ id: 9 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.find('article').findAll('button').find((button) => button.text() === 'admin.qualityOps.editShort')!.trigger('click')
    expect(wrapper.find('[data-testid="quality-auto-restore"]').attributes('aria-checked')).toBe('true')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(9, expect.objectContaining({
      expected_updated_at: '2026-10-31T00:00:00Z',
      pelican_config: expect.objectContaining({ quality: expect.objectContaining({ auto_restore: true }) })
    }))
    scroll.mockRestore()
  })

  it('loads the selected preset restore choice and saves it without carrying an older preset value', async () => {
    const preset = { name: 'check', model_id: 'm', cron_expression: '*/30 * * * *', enabled: true, max_results: 10, created_at: '', updated_at: '' }
    api.listPresets.mockResolvedValue([
      { ...preset, id: 1, pelican_config: { prompt: 'say 21', quality: { action: 'disable_scheduling', expected_answer: '21', auto_restore: true, judge: { kind: 'expected' } } } },
      { ...preset, id: 2, pelican_config: { prompt: 'say 21', quality: { action: 'disable_scheduling', expected_answer: '21', judge: { kind: 'expected' } } } }
    ])
    api.createPreset.mockResolvedValue({ ...preset, id: 3 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.find('[data-testid="quality-preset-select"]').setValue(1)
    await wrapper.find('[data-testid="quality-apply-preset"]').trigger('click')
    expect(wrapper.find('[data-testid="quality-auto-restore"]').attributes('aria-checked')).toBe('true')
    await wrapper.find('[data-testid="quality-toggle-save-preset"]').trigger('click')
    await wrapper.find('[data-testid="quality-preset-name"]').setValue('saved choice')
    await wrapper.find('[data-testid="quality-save-preset"]').trigger('click')
    await flushPromises()
    expect(api.createPreset).toHaveBeenCalledWith(expect.objectContaining({
      pelican_config: expect.objectContaining({ quality: expect.objectContaining({ auto_restore: true }) })
    }))
    await wrapper.find('[data-testid="quality-preset-select"]').setValue(2)
    await wrapper.find('[data-testid="quality-apply-preset"]').trigger('click')
    expect(wrapper.find('[data-testid="quality-auto-restore"]').attributes('aria-checked')).toBe('false')
  })

  it('preserves an existing template restore choice and defaults new templates to off', async () => {
    api.listTemplates.mockResolvedValue([{
      id: 8, account_filter: {}, model_id: 'm', cron_expression: '*/30 * * * *', enabled: true, max_results: 10, plan_ids: [], created_at: '', updated_at: '',
      pelican_config: { prompt: 'say 21', quality: { action: 'disable_scheduling', expected_answer: '21', auto_restore: true, judge: { kind: 'expected' } } }
    }])
    api.updateTemplate.mockResolvedValue({ updated: 0, created: 0, failed: 0 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.find('[data-testid="quality-template-row"]').findAll('button')[0].trigger('click')
    expect(wrapper.find('[data-testid="quality-template-auto-restore"]').attributes('aria-checked')).toBe('true')
    await wrapper.find('[data-testid="quality-template-form"]').trigger('submit')
    await flushPromises()
    expect(api.updateTemplate).toHaveBeenCalledWith(8, expect.objectContaining({
      pelican_config: expect.objectContaining({ quality: expect.objectContaining({ auto_restore: true }) })
    }))
    await wrapper.find('[data-testid="quality-template-new"]').trigger('click')
    expect(wrapper.find('[data-testid="quality-template-auto-restore"]').attributes('aria-checked')).toBe('false')
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
    await form.find('[data-testid="quality-template-model-id"]').setValue('gpt-5.1')
    await form.findAll('textarea')[0].setValue('say 21')
    await form.find('[data-testid="quality-template-expected-answer"]').setValue('21')
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

  it('preserves all configured models and repeated probes when an existing rule is edited', async () => {
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    api.listPlans.mockResolvedValue([{
      id: 9, account_id: 7, model_id: 'primary', cron_expression: '*/30 * * * *', enabled: true, max_results: 10,
      updated_at: '2026-10-31T00:00:00Z',
      pelican_config: { model_ids: ['primary', 'secondary'], parallel_count: 3, prompt: 'say 21', quality: { action: 'observe_only', expected_answer: '21', judge: { kind: 'expected' } } }
    }])
    api.update.mockResolvedValue({ id: 9 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.find('article').findAll('button').find((button) => button.text() === 'admin.qualityOps.editShort')!.trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(9, expect.objectContaining({
      expected_updated_at: '2026-10-31T00:00:00Z',
      pelican_config: expect.objectContaining({ model_ids: ['primary', 'secondary'], parallel_count: 3 })
    }))
    scroll.mockRestore()
  })

  it('loads and saves preset probe models without carrying values from a previous preset', async () => {
    const preset = { name: 'check', model_id: 'primary', cron_expression: '*/30 * * * *', enabled: true, max_results: 10, created_at: '', updated_at: '' }
    api.listPresets.mockResolvedValue([
      { ...preset, id: 1, pelican_config: { model_ids: ['primary', 'secondary'], parallel_count: 2, prompt: 'say 21', quality: { action: 'observe_only', expected_answer: '21', judge: { kind: 'expected' } } } },
      { ...preset, id: 2, pelican_config: { prompt: 'say 21', quality: { action: 'observe_only', expected_answer: '21', judge: { kind: 'expected' } } } }
    ])
    api.createPreset.mockResolvedValue({ ...preset, id: 3 })
    api.create.mockResolvedValue({ id: 8 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.find('[data-testid="quality-preset-select"]').setValue(1)
    await wrapper.find('[data-testid="quality-apply-preset"]').trigger('click')
    await wrapper.find('[data-testid="quality-toggle-save-preset"]').trigger('click')
    await wrapper.find('[data-testid="quality-preset-name"]').setValue('saved models')
    await wrapper.find('[data-testid="quality-save-preset"]').trigger('click')
    await flushPromises()
    expect(api.createPreset).toHaveBeenCalledWith(expect.objectContaining({
      pelican_config: expect.objectContaining({ model_ids: ['primary', 'secondary'], parallel_count: 2 })
    }))
    await wrapper.find('[data-testid="quality-preset-select"]').setValue(2)
    await wrapper.find('[data-testid="quality-apply-preset"]').trigger('click')
    await wrapper.findAll('select')[1].setValue(7)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({
      pelican_config: expect.objectContaining({ model_ids: ['primary'], parallel_count: 1 })
    }))
  })

  it('preserves a template model set and repeated probes when saving edits', async () => {
    api.listTemplates.mockResolvedValue([{
      id: 8, account_filter: {}, model_id: 'primary', cron_expression: '*/30 * * * *', enabled: true, max_results: 10, plan_ids: [], created_at: '', updated_at: '',
      pelican_config: { model_ids: ['primary', 'secondary'], parallel_count: 3, prompt: 'say 21', quality: { action: 'observe_only', expected_answer: '21', judge: { kind: 'expected' } } }
    }])
    api.updateTemplate.mockResolvedValue({ updated: 0, created: 0, failed: 0 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.find('[data-testid="quality-template-row"]').findAll('button')[0].trigger('click')
    await wrapper.find('[data-testid="quality-template-form"]').trigger('submit')
    await flushPromises()
    expect(api.updateTemplate).toHaveBeenCalledWith(8, expect.objectContaining({
      pelican_config: expect.objectContaining({ model_ids: ['primary', 'secondary'], parallel_count: 3 })
    }))
  })

  it('identifies each model and round in probe history and shows skipped samples', async () => {
    api.listHistory.mockResolvedValue({ items: [{ ...historyItem(100), total_count: 3, passed_count: 2, skipped_count: 1 }], next_cursor: 0 })
    api.listPlans.mockResolvedValue([{ id: 9, account_id: 7, model_id: 'primary', cron_expression: '*/30 * * * *', enabled: true, pelican_config: { model_ids: ['primary', 'secondary'], parallel_count: 2, quality: { action: 'observe_only' } } }])
    api.listResults.mockResolvedValue([
      { ...historyItem(99), quality_round_id: 'round-shared', pelican_config: { model_id: 'primary' } },
      { ...historyItem(98), quality_round_id: 'round-shared', pelican_config: { model_id: 'secondary' } }
    ])
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('admin.qualityOps.skippedCount')
    await wrapper.find('article').findAll('button').find((button) => button.text() === 'admin.qualityOps.history')!.trigger('click')
    await flushPromises()
    const resultRows = wrapper.findAll('[data-testid="quality-probe-result"]')
    expect(resultRows).toHaveLength(2)
    expect(resultRows[0].text()).toContain('primary')
    expect(resultRows[1].text()).toContain('secondary')
    expect(resultRows.every((row) => row.text().includes('round-shared'))).toBe(true)
  })

  it('creates a canonical model set from the primary and additional fields and resets after save', async () => {
    api.create.mockResolvedValue({ id: 5 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.findAll('select')[1].setValue(7)
    await wrapper.find('[data-testid="quality-model-id"]').setValue(' primary ')
    for (const model of [' secondary ', 'primary', 'secondary']) {
      await wrapper.find('[data-testid="quality-add-model"]').trigger('click')
      await wrapper.findAll('[data-testid="quality-extra-model"]').slice(-1)[0].setValue(model)
    }
    await wrapper.find('[data-testid="quality-parallel-count"]').setValue(3)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({
      model_id: 'primary',
      pelican_config: expect.objectContaining({ model_ids: ['primary', 'secondary'], parallel_count: 3 })
    }))
    expect(wrapper.findAll('[data-testid="quality-extra-model"]')).toHaveLength(0)
    expect((wrapper.find('[data-testid="quality-parallel-count"]').element as HTMLInputElement).value).toBe('1')
  })

  it('rejects an excessive sample round without clamping input and permits a corrected save', async () => {
    api.create.mockResolvedValue({ id: 5 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.findAll('select')[1].setValue(7)
    await wrapper.find('[data-testid="quality-model-id"]').setValue('primary')
    for (const model of ['second', 'third', 'fourth']) {
      await wrapper.find('[data-testid="quality-add-model"]').trigger('click')
      await wrapper.findAll('[data-testid="quality-extra-model"]').slice(-1)[0].setValue(model)
    }
    await wrapper.find('[data-testid="quality-parallel-count"]').setValue(32)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.create).not.toHaveBeenCalled()
    expect(wrapper.find('[role="alert"]').text()).toContain('admin.qualityOps.probeSamplesInvalid')
    expect((wrapper.find('[data-testid="quality-parallel-count"]').element as HTMLInputElement).value).toBe('32')
    expect(wrapper.findAll('[data-testid="quality-extra-model"]')).toHaveLength(3)
    await wrapper.find('[data-testid="quality-parallel-count"]').setValue(25)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({
      pelican_config: expect.objectContaining({ model_ids: ['primary', 'second', 'third', 'fourth'], parallel_count: 25 })
    }))
  })

  it('shows the legacy zero probe count as one and allows removing an additional model', async () => {
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    api.listPlans.mockResolvedValue([{
      id: 9, account_id: 7, model_id: 'primary', cron_expression: '*/30 * * * *', enabled: true, max_results: 10,
      updated_at: '2026-10-31T00:00:00Z',
      pelican_config: { model_ids: ['primary', 'secondary'], parallel_count: 0, prompt: 'say 21', quality: { action: 'observe_only', expected_answer: '21', judge: { kind: 'expected' } } }
    }])
    api.update.mockResolvedValue({ id: 9 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    await wrapper.find('article').findAll('button')[0].trigger('click')
    expect((wrapper.find('[data-testid="quality-parallel-count"]').element as HTMLInputElement).value).toBe('1')
    await wrapper.find('[data-testid="quality-extra-models"] button[aria-label]').trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(9, expect.objectContaining({
      pelican_config: expect.objectContaining({ model_ids: ['primary'], parallel_count: 1 })
    }))
    scroll.mockRestore()
  })

  it.each(['rule', 'preset', 'template'])('preserves explicit model targets instead of adding a stale primary when editing a %s', async (kind) => {
    const config = { model_ids: ['actual-a', 'actual-b'], parallel_count: 2, prompt: 'say 21', quality: { action: 'observe_only', expected_answer: '21', judge: { kind: 'expected' } } }
    const common = { model_id: 'legacy', cron_expression: '*/30 * * * *', enabled: true, max_results: 10, pelican_config: config }
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    if (kind === 'rule') api.listPlans.mockResolvedValue([{ ...common, id: 9, account_id: 7, updated_at: '2026-10-31T00:00:00Z' }])
    if (kind === 'preset') api.listPresets.mockResolvedValue([{ ...common, id: 1, name: 'explicit models', created_at: '', updated_at: '' }])
    if (kind === 'template') api.listTemplates.mockResolvedValue([{ ...common, id: 8, account_filter: {}, plan_ids: [], created_at: '', updated_at: '' }])
    api.update.mockResolvedValue({ id: 9 })
    api.create.mockResolvedValue({ id: 10 })
    api.updateTemplate.mockResolvedValue({ created: 0, updated: 1, failed: 0 })
    const wrapper = mount(AccountQualityView, { global: { stubs } })
    await flushPromises()
    if (kind === 'rule') {
      expect(wrapper.find('article').text()).toContain('actual-a / actual-b')
      expect(wrapper.find('article').text()).not.toContain('legacy')
      await wrapper.find('article').findAll('button')[0].trigger('click')
    } else if (kind === 'preset') {
      await wrapper.find('[data-testid="quality-preset-select"]').setValue(1)
      await wrapper.find('[data-testid="quality-apply-preset"]').trigger('click')
      await wrapper.findAll('select')[1].setValue(7)
    } else {
      const row = wrapper.find('[data-testid="quality-template-row"]')
      expect(row.text()).toContain('actual-a / actual-b')
      expect(row.text()).not.toContain('legacy')
      await row.findAll('button')[0].trigger('click')
    }
    const primary = kind === 'template' ? '[data-testid="quality-template-model-id"]' : '[data-testid="quality-model-id"]'
    expect((wrapper.find(primary).element as HTMLInputElement).value).toBe('actual-a')
    const form = kind === 'template' ? wrapper.find('[data-testid="quality-template-form"]') : wrapper.find('form')
    await form.trigger('submit')
    await flushPromises()
    const request = kind === 'template' ? api.updateTemplate.mock.calls[0][1] : kind === 'rule' ? api.update.mock.calls[0][1] : api.create.mock.calls[0][0]
    expect(request).toEqual(expect.objectContaining({ model_id: 'actual-a', pelican_config: expect.objectContaining({ model_ids: ['actual-a', 'actual-b'], parallel_count: 2 }) }))
    scroll.mockRestore()
  })

  it('preserves isolation group scope when editing and refuses to save it without groups', async () => {
    vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    api.listPlans.mockResolvedValue([{ id:9,account_id:7,model_id:'sol',cron_expression:'*/30 * * * *',enabled:true,max_results:100,updated_at:'2026-11-07T00:00:00Z',pelican_config:{model_ids:['sol','astra'],prompt:'probe',quality:{action:'isolate_models',remove_group_ids:[10],expected_answer:'ok',judge:{kind:'expected'}}} }])
    const wrapper=mount(AccountQualityView,{global:{stubs}})
    await flushPromises()
    await wrapper.find('article').findAll('button').find(b=>b.text()==='admin.qualityOps.editShort')!.trigger('click')
    await wrapper.find('form').trigger('submit');await flushPromises()
    expect(api.update).toHaveBeenCalledWith(9,expect.objectContaining({pelican_config:expect.objectContaining({quality:expect.objectContaining({action:'isolate_models',remove_group_ids:[10]})})}))
    await wrapper.find('article').findAll('button').find(b=>b.text()==='admin.qualityOps.editShort')!.trigger('click')
    await wrapper.find('[data-testid="quality-isolation-groups"] input').setValue(false)
    api.update.mockClear()
    await wrapper.find('form').trigger('submit');await flushPromises()
    expect(api.update).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.qualityOps.isolation.groupsRequired')
  })

  it('re-reads both rule scope and ownership after a single explicit model release', async () => {
    const plan={id:9,account_id:7,model_id:'sol',cron_expression:'*/30 * * * *',enabled:true,max_results:100,pelican_config:{model_ids:['sol','astra'],quality:{action:'isolate_models'}}}
    api.listPlans.mockResolvedValueOnce([plan]).mockResolvedValue([{...plan,model_id:'astra',pelican_config:{...plan.pelican_config,model_ids:['astra']}}])
    const view={plan_id:9,account_id:7,fence:'a'.repeat(64),state_present:true,action:'isolate_models',model_removals:[{model:'sol',isolation_model:'sol',ready:true,effect:'remove_orphan_marker_only'},{model:'astra',isolation_model:'astra',ready:false,effect:'remove_matching_marker',blocked_reason:'changed_same_owner'}]}
    api.ownershipPreview.mockResolvedValueOnce(view).mockResolvedValue({...view,model_removals:[view.model_removals[1]]})
    const confirm=vi.spyOn(window,'confirm').mockReturnValue(true)
    const scroll=vi.spyOn(window,'scrollTo').mockImplementation(()=>{})
    const wrapper=mount(AccountQualityView,{global:{stubs}});await flushPromises()
    await wrapper.find('article').findAll('button').find(b=>b.text()==='admin.qualityOps.editShort')!.trigger('click')
    await wrapper.find('[data-testid="quality-cron-expression"]').setValue('0 * * * *')
    await wrapper.findAll('button').find(b=>b.text()==='admin.qualityOps.ownership')!.trigger('click');await flushPromises()
    expect(wrapper.text()).toContain('admin.qualityOps.isolation.blocked.changed_same_owner')
    expect(wrapper.findAll('[data-testid="quality-remove-owned-model"]')[1].attributes('disabled')).toBeDefined()
    await wrapper.find('[data-testid="quality-remove-owned-model"]').trigger('click');await flushPromises()
    expect(api.removeOwnedModel).not.toHaveBeenCalled()
    await wrapper.find('[data-testid="quality-removal-reason"]').setValue('orphan_marker_recovery')
    await wrapper.find('[data-testid="quality-remove-owned-model"]').trigger('click');await flushPromises()
    expect(confirm).toHaveBeenCalledOnce()
    expect(api.removeOwnedModel).toHaveBeenCalledWith(9,{fence:'a'.repeat(64),confirm:true,model:'sol',reason_code:'orphan_marker_recovery'})
    expect(api.listPlans).toHaveBeenCalledTimes(2)
    expect(api.ownershipPreview).toHaveBeenCalledTimes(2)
    expect(wrapper.findAll('[data-testid="quality-remove-owned-model"]')).toHaveLength(1)
    expect((wrapper.find('[data-testid="quality-model-id"]').element as HTMLInputElement).value).toBe('astra')
    expect((wrapper.find('[data-testid="quality-cron-expression"]').element as HTMLInputElement).value).toBe('0 * * * *')
    confirm.mockRestore()
    scroll.mockRestore()
  })

  it.each(['rule-read-error', 'preview-read-error', 'stale-rule-read'])('keeps drafts and reports an unverified committed release on %s', async (failure) => {
    const plan={id:9,account_id:7,model_id:'sol',cron_expression:'*/30 * * * *',enabled:true,max_results:100,pelican_config:{model_ids:['sol','astra'],quality:{action:'isolate_models'}}}
    const view={plan_id:9,account_id:7,fence:'a'.repeat(64),state_present:true,action:'isolate_models',model_removals:[{model:'sol',isolation_model:'sol',ready:true,effect:'remove_orphan_marker_only'}]}
    api.listPlans.mockResolvedValueOnce([plan])
    if(failure==='rule-read-error') api.listPlans.mockRejectedValue(new Error('read unavailable'))
    else api.listPlans.mockResolvedValue([failure==='stale-rule-read'?plan:{...plan,model_id:'astra',pelican_config:{...plan.pelican_config,model_ids:['astra']}}])
    api.ownershipPreview.mockResolvedValueOnce(view)
    if(failure==='preview-read-error') api.ownershipPreview.mockRejectedValue(new Error('preview unavailable'))
    else api.ownershipPreview.mockResolvedValue({...view,model_removals:[]})
    const confirm=vi.spyOn(window,'confirm').mockReturnValue(true)
    const scroll=vi.spyOn(window,'scrollTo').mockImplementation(()=>{})
    const wrapper=mount(AccountQualityView,{global:{stubs}});await flushPromises()
    await wrapper.find('article').findAll('button').find(b=>b.text()==='admin.qualityOps.editShort')!.trigger('click')
    await wrapper.findAll('button').find(b=>b.text()==='admin.qualityOps.ownership')!.trigger('click');await flushPromises()
    await wrapper.find('[data-testid="quality-removal-reason"]').setValue('retire_model')
    await wrapper.find('[data-testid="quality-remove-owned-model"]').trigger('click');await flushPromises()
    expect(api.removeOwnedModel).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('admin.qualityOps.isolation.readbackFailed')
    expect(wrapper.text()).not.toContain('admin.qualityOps.operationDone')
    expect((wrapper.find('[data-testid="quality-model-id"]').element as HTMLInputElement).value).toBe('sol')
    confirm.mockRestore();scroll.mockRestore()
  })

  it('refreshes stale previews on conflict and shows the original error without retrying a mutation', async () => {
    api.listPlans.mockResolvedValue([{id:9,account_id:7,model_id:'sol',pelican_config:{quality:{action:'isolate_models'}}}])
    const view={plan_id:9,fence:'b'.repeat(64),state_present:true,model_removals:[{model:'sol',isolation_model:'sol',ready:true,effect:'remove_orphan_marker_only'}]}
    api.ownershipPreview.mockResolvedValue(view)
    api.removeOwnedModel.mockRejectedValue(new Error('preview changed'))
    const confirm=vi.spyOn(window,'confirm').mockReturnValue(true)
    const wrapper=mount(AccountQualityView,{global:{stubs}});await flushPromises()
    await wrapper.findAll('button').find(b=>b.text()==='admin.qualityOps.ownership')!.trigger('click');await flushPromises()
    await wrapper.find('[data-testid="quality-removal-reason"]').setValue('retire_model')
    await wrapper.find('[data-testid="quality-remove-owned-model"]').trigger('click');await flushPromises()
    expect(api.removeOwnedModel).toHaveBeenCalledOnce()
    expect(api.ownershipPreview).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('preview changed')
    expect(wrapper.find('[data-testid="quality-removal-reason"]').element.value).toBe('')
    confirm.mockRestore()
  })

  it('retains saved isolation scopes in presets and group templates', async () => {
    const config={model_ids:['sol','astra'],prompt:'probe',quality:{action:'isolate_models',remove_group_ids:[10],expected_answer:'ok',judge:{kind:'expected'}}}
    api.listPresets.mockResolvedValue([{id:5,name:'isolation preset',model_id:'sol',cron_expression:'*/30 * * * *',max_results:100,enabled:true,pelican_config:config}])
    api.listTemplates.mockResolvedValue([{id:6,model_id:'sol',cron_expression:'*/30 * * * *',max_results:100,enabled:true,account_filter:{},plan_ids:[],pelican_config:config}])
    const wrapper=mount(AccountQualityView,{global:{stubs}});await flushPromises()
    await wrapper.find('[data-testid="quality-preset-select"]').setValue(5)
    await wrapper.find('[data-testid="quality-apply-preset"]').trigger('click')
    await wrapper.findAll('select')[1].setValue(7)
    await wrapper.find('form').trigger('submit');await flushPromises()
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({pelican_config:expect.objectContaining({quality:expect.objectContaining({remove_group_ids:[10],action:'isolate_models'})})}))
    await wrapper.find('[data-testid="quality-template-row"]').findAll('button')[0].trigger('click')
    await wrapper.find('[data-testid="quality-template-form"]').trigger('submit');await flushPromises()
    expect(api.updateTemplate).toHaveBeenCalledWith(6,expect.objectContaining({pelican_config:expect.objectContaining({quality:expect.objectContaining({remove_group_ids:[10],action:'isolate_models'})})}))
  })

  it('retains selected groups on a load failure and blocks isolation writes until groups can be loaded', async () => {
    api.groupsAll.mockRejectedValue(new Error('group network error'))
    api.listPresets.mockResolvedValue([{id:5,name:'saved',model_id:'sol',cron_expression:'*/30 * * * *',max_results:100,enabled:true,pelican_config:{model_ids:['sol'],prompt:'probe',quality:{action:'isolate_models',remove_group_ids:[10],expected_answer:'ok'}}}])
    const wrapper=mount(AccountQualityView,{global:{stubs}});await flushPromises()
    await wrapper.find('[data-testid="quality-preset-select"]').setValue(5)
    await wrapper.find('[data-testid="quality-apply-preset"]').trigger('click')
    expect(wrapper.text()).toContain('group network error')
    expect(wrapper.find('[data-testid="quality-isolation-groups"] input').element.checked).toBe(true)
    await wrapper.findAll('select')[1].setValue(7)
    await wrapper.find('form').trigger('submit');await flushPromises()
    expect(api.create).not.toHaveBeenCalled()
  })

})
