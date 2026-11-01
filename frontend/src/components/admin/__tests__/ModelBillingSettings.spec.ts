import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelBillingSettings from '../ModelBillingSettings.vue'

const { getConfig, saveConfig, getGroups } = vi.hoisted(() => ({ getConfig: vi.fn(), saveConfig: vi.fn(), getGroups: vi.fn() }))
vi.mock('@/api/admin/modelBilling', () => ({ getModelBillingConfig: getConfig, saveModelBillingConfig: saveConfig }))
vi.mock('@/api/admin/groups', () => ({ getAll: getGroups }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

function render() { return mount(ModelBillingSettings) }
async function editRule(wrapper: ReturnType<typeof render>, model = 'gpt-6*', multiplier = '2.5') {
  await wrapper.get('[data-testid="add-rule"]').trigger('click')
  await wrapper.get('[data-testid="rule-model"]').setValue(model)
  await wrapper.get('[data-testid="rule-multiplier"]').setValue(multiplier)
}

describe('ModelBillingSettings', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    getConfig.mockResolvedValue({ config: { enabled: false, rules: null }, revision: '' })
    getGroups.mockResolvedValue([{ id: 7, name: 'Active group' }])
    saveConfig.mockImplementation(async (value) => ({ ...value, revision: 'r2' }))
  })

  it('loads disabled defaults and active groups, and saves explicit global scope and revision', async () => {
    const wrapper = render()
    expect(wrapper.attributes('aria-busy')).toBe('true')
    await flushPromises()
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.text()).toContain('admin.settings.modelBilling.empty')
    await wrapper.get('[role="switch"]').trigger('click')
    await editRule(wrapper)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(saveConfig).toHaveBeenCalledWith({ config: { enabled: true, rules: [{ model: 'gpt-6*', multiplier: 2.5, group_ids: [] }] }, revision: '' })
    expect(wrapper.get('[role="status"]').text()).toContain('admin.settings.modelBilling.saved')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(saveConfig.mock.lastCall?.[0].revision).toBe('r2')
  })

  it('requires explicit selected groups and round trips their IDs', async () => {
    const wrapper = render()
    await flushPromises()
    await editRule(wrapper)
    await wrapper.get('[data-testid="rule-scope"]').setValue('selected')
    await wrapper.get('form').trigger('submit')
    expect(saveConfig).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="rule-scope"]').attributes('aria-invalid')).toBe('true')
    await wrapper.get('input[type="checkbox"][value="7"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(saveConfig.mock.lastCall?.[0].config.rules[0].group_ids).toEqual([7])
  })

  it.each([['g*pt', '2'], ['gpt', '0.5'], ['gpt', '1001'], ['', '2']])('rejects invalid model/multiplier %s / %s', async (model, multiplier) => {
    const wrapper = render()
    await flushPromises()
    await editRule(wrapper, model, multiplier)
    await wrapper.get('form').trigger('submit')
    expect(saveConfig).not.toHaveBeenCalled()
    expect(wrapper.find('[aria-invalid="true"]').exists()).toBe(true)
  })

  it('rejects duplicate case-insensitive models and enabled configuration without rules', async () => {
    getConfig.mockResolvedValue({ config: { enabled: true, rules: [] }, revision: 'r1' })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    expect(saveConfig).not.toHaveBeenCalled()
    await editRule(wrapper, 'GPT-6')
    await wrapper.get('[data-testid="add-rule"]').trigger('click')
    await wrapper.findAll('[data-testid="rule-model"]')[1]!.setValue(' gpt-6 ')
    await wrapper.get('form').trigger('submit')
    expect(saveConfig).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.settings.modelBilling.invalidModel')
  })

  it('caps the editor at 100 rules and allows removing a rule before adding another', async () => {
    getConfig.mockResolvedValue({ config: { enabled: false, rules: Array.from({ length: 100 }, (_, i) => ({ model: `gpt-${i}`, multiplier: 1, group_ids: [] })) }, revision: 'r1' })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="add-rule"]').attributes('disabled')).toBeDefined()
    await wrapper.get('button[aria-label="admin.settings.modelBilling.removeRule 1"]').trigger('click')
    expect(wrapper.findAll('[data-testid="rule-model"]')).toHaveLength(99)
    expect(wrapper.get('[data-testid="add-rule"]').attributes('disabled')).toBeUndefined()
  })

  it('keeps edited values on 409, prevents stale retries, and reloads only on explicit discard', async () => {
    saveConfig.mockRejectedValue({ status: 409, code: 'MODEL_BILLING_REVISION_CONFLICT' })
    const wrapper = render()
    await flushPromises()
    await editRule(wrapper)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect((wrapper.get('[data-testid="rule-model"]').element as HTMLInputElement).value).toBe('gpt-6*')
    expect(getConfig).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="save-config"]').attributes('disabled')).toBeDefined()
    getConfig.mockResolvedValue({ config: { enabled: true, rules: [{ model: 'claude-*', multiplier: 4, group_ids: [7] }] }, revision: 'r3' })
    await wrapper.get('[data-testid="reload-config"]').trigger('click')
    await flushPromises()
    expect((wrapper.get('[data-testid="rule-model"]').element as HTMLInputElement).value).toBe('claude-*')
    expect((wrapper.get('input[value="7"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('form').trigger('submit')
    expect(saveConfig.mock.lastCall?.[0].revision).toBe('r3')
  })

  it('preserves and exposes inactive saved scopes instead of turning them global', async () => {
    getConfig.mockResolvedValue({ config: { enabled: true, rules: [{ model: 'gpt', multiplier: 2, group_ids: [99] }] }, revision: 'r1' })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('#99')
    await wrapper.get('form').trigger('submit')
    expect(saveConfig).not.toHaveBeenCalled()
    await wrapper.get('input[value="99"]').setValue(false)
    await wrapper.get('input[value="7"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(saveConfig.mock.lastCall?.[0].config.rules[0].group_ids).toEqual([7])
  })

  it('allows disabling while preserving an inactive saved scope', async () => {
    getConfig.mockResolvedValue({ config: { enabled: true, rules: [{ model: 'gpt', multiplier: 2, group_ids: [99] }] }, revision: 'r1' })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(saveConfig).toHaveBeenCalledWith({ config: { enabled: false, rules: [{ model: 'gpt', multiplier: 2, group_ids: [99] }] }, revision: 'r1' })
    expect((wrapper.get('input[value="99"]').element as HTMLInputElement).checked).toBe(true)
  })

  it('blocks writes after load failure and allows an explicit retry', async () => {
    getGroups.mockRejectedValueOnce({ message: 'Groups unavailable' })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('Groups unavailable')
    expect(wrapper.find('form').exists()).toBe(false)
    await wrapper.get('[data-testid="reload-config"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('preserves input after ordinary save failure and disables controls while saving', async () => {
    let rejectSave!: (reason: unknown) => void
    saveConfig.mockImplementation(() => new Promise((_, reject) => { rejectSave = reject }))
    const wrapper = render()
    await flushPromises()
    await editRule(wrapper)
    await wrapper.get('form').trigger('submit')
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    rejectSave({ message: 'Save unavailable' })
    await flushPromises()
    expect((wrapper.get('[data-testid="rule-model"]').element as HTMLInputElement).value).toBe('gpt-6*')
    expect(wrapper.text()).toContain('Save unavailable')
    expect(wrapper.get('[data-testid="save-config"]').attributes('disabled')).toBeUndefined()
  })
})
