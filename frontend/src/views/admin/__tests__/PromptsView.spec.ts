import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PromptsView from '@/views/admin/PromptsView.vue'

const { getPromptMock, listPromptsMock, updatePromptMock } = vi.hoisted(() => ({
  getPromptMock: vi.fn(),
  listPromptsMock: vi.fn(),
  updatePromptMock: vi.fn(),
}))

vi.mock('@/api/admin/prompts', () => ({
  listAdminPrompts: (...args: unknown[]) => listPromptsMock(...args),
  getAdminPrompt: (...args: unknown[]) => getPromptMock(...args),
  listImportItems: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 }),
  listReports: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 }),
  createAdminPrompt: vi.fn(),
  createPromptImportJob: vi.fn(),
  updateAdminPrompt: (...args: unknown[]) => updatePromptMock(...args),
  deleteAdminPrompt: vi.fn(),
  submitPromptReview: vi.fn(),
  approvePrompt: vi.fn(),
  publishPrompt: vi.fn(),
  unpublishPrompt: vi.fn(),
  rollbackPrompt: vi.fn(),
  approveImportItem: vi.fn(),
  rejectImportItem: vi.fn(),
  resolvePromptReport: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

describe('PromptsView', () => {
  beforeEach(() => {
    getPromptMock.mockReset()
    listPromptsMock.mockReset().mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0,
    })
    updatePromptMock.mockReset().mockResolvedValue({})
  })

  it('uses a fully Chinese admin surface and defaults new external content to 极速蹬精选', async () => {
    const wrapper = mount(PromptsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          BaseDialog: {
            props: ['show'],
            template: '<div v-if="show"><slot /><slot name="footer" /></div>',
          },
          ConfirmDialog: true,
          Pagination: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('提示词管理')
    expect(wrapper.text()).toContain('导入待审')
    expect(wrapper.text()).toContain('投诉处理')

    await wrapper.get('[data-testid="create-prompt"]').trigger('click')
    const sourceSelect = wrapper.get('[data-testid="prompt-source-attribution"]')
    expect((sourceSelect.element as HTMLSelectElement).value).toBe('curated')
    expect(sourceSelect.text()).toContain('极速蹬精选')
    expect(sourceSelect.text()).toContain('极速蹬原创')
  })

  it('loads the current detail before editing and saves with that current version', async () => {
    const summary = {
      id: '41',
      title: '列表摘要标题',
      purpose_description: '列表摘要',
      prompt_template: 'summary prompt',
      variables: [],
      recommended_models: [],
      recommended_sizes: [],
      reference_requirement: 'none',
      source_attribution: 'curated',
      featured: false,
      version: 3,
      status: 'draft',
    }
    const detail = {
      ...summary,
      title: '详情中的当前标题',
      purpose_description: '详情中的完整用途',
      prompt_template: 'detail prompt',
      version: 7,
    }
    listPromptsMock.mockResolvedValueOnce({
      items: [summary],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getPromptMock.mockResolvedValueOnce(detail)

    const wrapper = mount(PromptsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          BaseDialog: {
            props: ['show'],
            template: '<div v-if="show"><slot /><slot name="footer" /></div>',
          },
          ConfirmDialog: true,
          Pagination: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('.prompt-admin-action').trigger('click')
    await flushPromises()

    expect(getPromptMock).toHaveBeenCalledWith('41')
    expect((wrapper.get('input[required]').element as HTMLInputElement).value)
      .toBe('详情中的当前标题')

    await wrapper.get('#prompt-editor-form').trigger('submit')
    await flushPromises()

    expect(updatePromptMock).toHaveBeenCalledWith(
      '41',
      expect.objectContaining({
        title: '详情中的当前标题',
        prompt_template: 'detail prompt',
      }),
      7,
    )
  })
})
