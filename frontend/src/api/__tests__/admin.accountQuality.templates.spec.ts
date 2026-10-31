import { describe, expect, it, vi } from 'vitest'
import { createTemplate, deleteTemplate, listTemplates, updateTemplate } from '../admin/accountQuality'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }))
vi.mock('../client', () => ({ apiClient: client }))

describe('admin quality template API', () => {
  it('uses admin-only template routes for list and create', async () => {
    client.get.mockResolvedValueOnce({ data: [] })
    client.post.mockResolvedValueOnce({ data: { id: 4 } })
    await listTemplates()
    await createTemplate({ model_id: 'gpt-5.1' })
    expect(client.get).toHaveBeenCalledWith('/admin/account-quality-templates')
    expect(client.post).toHaveBeenCalledWith('/admin/account-quality-templates', { model_id: 'gpt-5.1' })
  })

  it('keeps plan deletion explicit in the admin request', async () => {
    client.put.mockResolvedValueOnce({ data: { updated: 1, created: 0, failed: 0 } })
    client.delete.mockResolvedValueOnce({ data: { deleted: 2 } })
    await updateTemplate(4, { enabled: false })
    await deleteTemplate(4, true)
    expect(client.put).toHaveBeenCalledWith('/admin/account-quality-templates/4', { enabled: false })
    expect(client.delete).toHaveBeenCalledWith('/admin/account-quality-templates/4', { params: { delete_plans: 'true' } })
  })
})
