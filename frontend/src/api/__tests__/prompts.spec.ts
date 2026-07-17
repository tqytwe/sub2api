import { beforeEach, describe, expect, it, vi } from 'vitest'

const { deleteMock, postMock } = vi.hoisted(() => ({
  deleteMock: vi.fn(),
  postMock: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    delete: deleteMock,
    post: postMock,
  },
}))

import {
  favoritePrompt,
  normalizePromptCategory,
  normalizePromptUseResult,
  normalizePublicPrompt,
  unfavoritePrompt,
} from '@/api/prompts'

describe('提示词公开接口转换', () => {
  beforeEach(() => {
    deleteMock.mockReset()
    postMock.mockReset()
  })

  it('converts the backend public contract without exposing source evidence', () => {
    const prompt = normalizePublicPrompt({
      id: 123,
      title: '电商产品主图',
      description: '用于生成干净的商品展示图。',
      purpose: '电商主图',
      style: '极简',
      subject: '产品',
      featured: true,
      version: 4,
      prompt_text: 'Product photo of {{product}}',
      variables: {
        product: { label: '产品名称', required: true, description: '填写商品名称' },
      },
      models: ['gpt-image-1'],
      sizes: ['1024x1024'],
      requires_reference: false,
      brand_label: '极速蹬精选',
      content_notice: '收录于极速蹬提示词库',
      public_attribution_note: '原始作者信息见内容说明',
      use_count: 8,
      favorite_count: 3,
      favorited: true,
      media: [{ id: 9, url: 'https://img.example/1.jpg', alt_zh: '白底商品图' }],
      source_url: 'https://must-not-leak.example',
      evidence: { captured_at: '2026-07-17' },
    })

    expect(prompt).toEqual(expect.objectContaining({
      id: '123',
      title: '电商产品主图',
      purpose_description: '用于生成干净的商品展示图。',
      prompt_template: 'Product photo of {{product}}',
      variables: [{
        name: 'product',
        label: '产品名称',
        required: true,
        description: '填写商品名称',
      }],
      recommended_models: ['gpt-image-1'],
      recommended_sizes: ['1024x1024'],
      reference_requirement: 'none',
      source_attribution: 'curated',
      preview_image_url: 'https://img.example/1.jpg',
      is_favorited: true,
    }))
    expect(prompt).not.toHaveProperty('source_url')
    expect(prompt).not.toHaveProperty('evidence')
  })

  it('converts use and category results for the image studio and filters', () => {
    expect(normalizePromptUseResult({
      prompt_id: 123,
      version: 4,
      title: '电商产品主图',
      prompt_text: 'Product photo of {{product}}',
      variables: {
        product: { label: '产品名称', default_value: '水杯' },
      },
      models: ['gpt-image-1'],
      sizes: ['1024x1024'],
      requires_reference: true,
    })).toEqual({
      prompt_id: '123',
      version: 4,
      title: '电商产品主图',
      prompt_template: 'Product photo of {{product}}',
      variables: [{ name: 'product', label: '产品名称', default_value: '水杯' }],
      recommended_models: ['gpt-image-1'],
      recommended_sizes: ['1024x1024'],
      reference_requirement: 'required',
    })

    expect(normalizePromptCategory({
      id: 1,
      slug: 'style:minimal',
      name_zh: '极简',
      sort_order: 10,
    })).toEqual({
      id: 1,
      slug: 'minimal',
      name: '极简',
      dimension: 'style',
      sort_order: 10,
    })
  })

  it('returns the final favorite state and optional server count', async () => {
    postMock.mockResolvedValueOnce({
      data: { prompt_id: 123, favorited: true, favorite_count: 9 },
    })
    deleteMock.mockResolvedValueOnce({
      data: { prompt_id: 123, favorited: false },
    })

    await expect(favoritePrompt('123')).resolves.toEqual({
      favorited: true,
      favorite_count: 9,
    })
    await expect(unfavoritePrompt('123')).resolves.toEqual({
      favorited: false,
    })
  })
})
