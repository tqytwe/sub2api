import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import ModerationUsersSummary from '../ModerationUsersSummary.vue'

const { listModerationUsers, getModerationUser, manageModerationUser } = vi.hoisted(() => ({
  listModerationUsers: vi.fn(),
  getModerationUser: vi.fn(),
  manageModerationUser: vi.fn(),
}))

vi.mock('@/api/admin/riskControl', () => ({
  listModerationUsers,
  getModerationUser,
  manageModerationUser,
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

const state = (overrides: Record<string, unknown> = {}) => ({
  user_id: 42,
  version: 7,
  generation: 2,
  auto_ban_exempt: false,
  keyword_exempt: false,
  api_audit_exempt: false,
  email_exempt: false,
  owned_disable: true,
  status: 'disabled',
  current_count: 4,
  reason: 'automatic moderation',
  actor_id: null,
  updated_at: null,
  ...overrides,
})

const BaseDialogStub = defineComponent({
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show" data-test="user-dialog"><slot /></div>',
})

const mountSummary = () => mount(ModerationUsersSummary, { global: { stubs: { BaseDialog: BaseDialogStub } } })

describe('ModerationUsersSummary', () => {
  beforeEach(() => {
    listModerationUsers.mockReset().mockResolvedValue({ items: [state()], total: 1 })
    getModerationUser.mockReset().mockResolvedValue(state())
    manageModerationUser.mockReset()
  })

  it('searches by UID using the server list contract', async () => {
    const wrapper = mountSummary()
    await flushPromises()
    expect(listModerationUsers).toHaveBeenCalledWith({ page: 1, page_size: 20, state: 'all' })

    await wrapper.get('[data-test="uid-search"]').setValue('42')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(listModerationUsers).toHaveBeenLastCalledWith({ page: 1, page_size: 20, state: 'all', user_id: 42 })
    expect(wrapper.text()).toContain('42')
  })

  it('applies state filters and loads the requested page', async () => {
    listModerationUsers.mockResolvedValue({ items: [state()], total: 45 })
    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="state-filter"]').setValue('enforced')
    await flushPromises()
    expect(listModerationUsers).toHaveBeenLastCalledWith({ page: 1, page_size: 20, state: 'enforced' })
    await wrapper.get('[data-test="next-users-page"]').trigger('click')
    await flushPromises()
    expect(listModerationUsers).toHaveBeenLastCalledWith({ page: 2, page_size: 20, state: 'enforced' })
  })

  it('shows loading state until a list response arrives', async () => {
    let finish!: (value: unknown) => void
    listModerationUsers.mockImplementation(() => new Promise((resolve) => { finish = resolve }))
    const wrapper = mountSummary()
    await nextTick()
    expect(wrapper.get('[role="status"]').text()).toBe('common.loading')
    finish({ items: [state()], total: 1 })
    await flushPromises()
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="open-user-42"]').exists()).toBe(true)
  })

  it('requires the current owned-disable state, a reason and confirmation before sending the versioned unban', async () => {
    getModerationUser.mockResolvedValueOnce(state()).mockResolvedValueOnce(state({ version: 8, owned_disable: false, status: 'active' }))
    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="open-user-42"]').trigger('click')
    await flushPromises()
    expect(getModerationUser).toHaveBeenCalledWith(42)
    expect(wrapper.find('[data-test="unban-user"]').exists()).toBe(true)
    expect((wrapper.get('[data-test="unban-user"]').element as HTMLButtonElement).disabled).toBe(true)

    await wrapper.get('[data-test="unban-reason"]').setValue('Reviewed appeal')
    expect((wrapper.get('[data-test="unban-user"]').element as HTMLButtonElement).disabled).toBe(true)
    await wrapper.get('[data-test="unban-confirm"]').setValue(true)
    manageModerationUser.mockResolvedValue(state({ version: 8, owned_disable: false, status: 'active' }))
    await wrapper.get('[data-test="unban-user"]').trigger('click')
    await flushPromises()

    expect(manageModerationUser).toHaveBeenCalledWith(42, {
      action: 'unban', version: 7, reason: 'Reviewed appeal', confirm: true,
    })
    expect(getModerationUser).toHaveBeenLastCalledWith(42)
    expect(wrapper.text()).toContain('admin.moderationUsers.saved')
  })

  it('does not offer moderation unban when the disable belongs to another source', async () => {
    getModerationUser.mockResolvedValue(state({ owned_disable: false }))
    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="open-user-42"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="unban-user"]').exists()).toBe(false)
    expect(manageModerationUser).not.toHaveBeenCalled()
  })

  it('saves explicit exemption values and verifies the updated server state', async () => {
    getModerationUser.mockResolvedValueOnce(state()).mockResolvedValueOnce(state({ version: 8, keyword_exempt: true }))
    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="open-user-42"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="exemption-keyword_exempt"]').setValue(true)
    await wrapper.get('[data-test="unban-reason"]').setValue('Policy exception approved')
    await wrapper.get('[data-test="unban-confirm"]').setValue(true)
    manageModerationUser.mockResolvedValue(state({ version: 8, keyword_exempt: true }))
    expect((wrapper.get('[data-test="save-exemptions"]').element as HTMLButtonElement).disabled).toBe(false)
    await wrapper.get('[data-test="save-exemptions"]').trigger('click')
    await flushPromises()
    expect(manageModerationUser).toHaveBeenCalledWith(42, {
      action: 'exemption', version: 7, reason: 'Policy exception approved', confirm: true,
      auto_ban_exempt: false, keyword_exempt: true, api_audit_exempt: false, email_exempt: false,
    })
    expect(wrapper.text()).toContain('admin.moderationUsers.saved')
  })

  it('resets the current count with a version and confirms the zero count from the server', async () => {
    getModerationUser.mockResolvedValueOnce(state()).mockResolvedValueOnce(state({ version: 8, current_count: 0 }))
    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="open-user-42"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="unban-reason"]').setValue('Review period completed')
    await wrapper.get('[data-test="unban-confirm"]').setValue(true)
    manageModerationUser.mockResolvedValue(state({ version: 8, current_count: 0 }))
    await wrapper.get('[data-test="reset-count"]').trigger('click')
    await flushPromises()
    expect(manageModerationUser).toHaveBeenCalledWith(42, {
      action: 'reset', version: 7, reason: 'Review period completed', confirm: true,
    })
    expect(wrapper.text()).toContain('admin.moderationUsers.saved')
  })

  it('refreshes the state after a write and does not report success if readback fails', async () => {
    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="open-user-42"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="unban-reason"]').setValue('Reviewed appeal')
    await wrapper.get('[data-test="unban-confirm"]').setValue(true)
    manageModerationUser.mockResolvedValue(state({ version: 8, owned_disable: false, status: 'active' }))
    getModerationUser.mockResolvedValueOnce(state()).mockRejectedValueOnce(new Error('readback failed'))
    await wrapper.get('[data-test="unban-user"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.moderationUsers.readbackFailed')
    expect(wrapper.text()).not.toContain('admin.moderationUsers.saved')
  })

  it('refreshes the version after a conflict and requires confirmation again', async () => {
    getModerationUser.mockResolvedValueOnce(state()).mockResolvedValueOnce(state({ version: 8 }))
    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="open-user-42"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="unban-reason"]').setValue('Reviewed appeal')
    await wrapper.get('[data-test="unban-confirm"]').setValue(true)
    manageModerationUser.mockRejectedValue(new Error('version conflict'))
    await wrapper.get('[data-test="unban-user"]').trigger('click')
    await flushPromises()
    expect(manageModerationUser).toHaveBeenCalledWith(42, {
      action: 'unban', version: 7, reason: 'Reviewed appeal', confirm: true,
    })
    expect(getModerationUser).toHaveBeenLastCalledWith(42)
    expect(wrapper.text()).toContain('version conflict')
    expect((wrapper.get('[data-test="unban-confirm"]').element as HTMLInputElement).checked).toBe(false)
    expect((wrapper.get('[data-test="unban-user"]').element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.text()).not.toContain('admin.moderationUsers.saved')
  })

  it('does not allow switching users while a management action is pending', async () => {
    listModerationUsers.mockResolvedValue({ items: [state({ reason: 'user-42' }), state({ user_id: 43, reason: 'user-43' })], total: 2 })
    let finishAction!: (value: unknown) => void
    manageModerationUser.mockImplementation(() => new Promise((resolve) => { finishAction = resolve }))
    getModerationUser.mockResolvedValueOnce(state({ reason: 'user-42' })).mockResolvedValueOnce(state({ status: 'active', owned_disable: false, reason: 'user-42' }))

    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="open-user-42"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="unban-reason"]').setValue('Reviewed appeal')
    await wrapper.get('[data-test="unban-confirm"]').setValue(true)
    await wrapper.get('[data-test="unban-user"]').trigger('click')
    await nextTick()

    const otherUserButton = wrapper.get<HTMLButtonElement>('[data-test="open-user-43"]')
    expect(otherUserButton.element.disabled).toBe(true)
    await otherUserButton.trigger('click')
    expect(wrapper.get('[data-test="user-dialog"]').text()).toContain('user-42')
    expect(wrapper.get('[data-test="user-dialog"]').text()).not.toContain('user-43')

    finishAction(state({ status: 'active', owned_disable: false }))
    await flushPromises()
    expect(wrapper.get('[data-test="user-dialog"]').text()).toContain('user-42')
    expect(wrapper.get('[data-test="user-dialog"]').text()).not.toContain('user-43')
  })

  it('does not render a detail response for a different UID than the requested user', async () => {
    listModerationUsers.mockResolvedValue({ items: [state()], total: 1 })
    getModerationUser.mockResolvedValue(state({ user_id: 43, reason: 'wrong-user-state' }))

    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="open-user-42"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="user-dialog"] [role="alert"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="user-dialog"]').text()).not.toContain('wrong-user-state')
    expect(wrapper.get('[data-test="user-dialog"]').text()).not.toContain('admin.moderationUsers.moderationOwned')
  })

  it('keeps the newest detail when an older request completes later', async () => {
    listModerationUsers.mockResolvedValue({ items: [state({ reason: 'user-42' }), state({ user_id: 43, reason: 'user-43' })], total: 2 })
    let finishFirst!: (value: unknown) => void
    let finishSecond!: (value: unknown) => void
    getModerationUser
      .mockImplementationOnce(() => new Promise((resolve) => { finishFirst = resolve }))
      .mockImplementationOnce(() => new Promise((resolve) => { finishSecond = resolve }))

    const wrapper = mountSummary()
    await flushPromises()
    await wrapper.get('[data-test="open-user-42"]').trigger('click')
    await wrapper.get('[data-test="open-user-43"]').trigger('click')
    finishSecond(state({ user_id: 43, reason: 'user-43' }))
    await flushPromises()
    finishFirst(state({ user_id: 42, reason: 'user-42' }))
    await flushPromises()

    expect(wrapper.get('[data-test="user-dialog"]').text()).toContain('user-43')
    expect(wrapper.get('[data-test="user-dialog"]').text()).not.toContain('user-42')
  })
})
