import type { PromptSummary } from '@/api/prompts'

const genericTemplatePaths = new Set([
  '/image-studio/templates/ecom-white-bg.webp',
  '/image-studio/templates/free-create.webp',
  '/image-studio/templates/xhs-cover.webp',
])

const purposeLabels: Record<string, string> = {
  'ad-creative': '广告创意',
  'app-web-design': '界面设计',
  'document-presentation': '文档演示',
  'ecommerce-main-image': '电商主图',
  'free-creation': '自由创作',
  'game-asset': '游戏素材',
  'infographic-edu-visual': '信息图',
  'live-commerce': '直播电商',
  'packaging-design': '包装设计',
  'poster-typography': '海报字体',
  'profile-avatar': '头像形象',
  'social-media-post': '社媒内容',
  'virtual-try-on': '虚拟试穿',
  'youtube-thumbnail': '视频封面',
}

const styleLabels: Record<string, string> = {
  '3d-render': '3D',
  'cinematic-film-still': '电影感',
  'flat-vector': '矢量',
  'luxury-editorial': '高级感',
  'minimal-clean': '极简',
  photorealistic: '写实',
  'poster-typography': '字体',
  watercolor: '水彩',
}

export function isGenericPromptTemplateImage(url?: string): boolean {
  if (!url) return false

  try {
    const base = typeof window === 'undefined' ? 'https://jisudeng.local' : window.location.origin
    return genericTemplatePaths.has(new URL(url, base).pathname)
  } catch {
    return genericTemplatePaths.has(url)
  }
}

export function shouldUseGeneratedPromptCover(prompt: PromptSummary): boolean {
  return !prompt.preview_image_url || isGenericPromptTemplateImage(prompt.preview_image_url)
}

export function promptCoverTone(prompt: PromptSummary): string {
  const key = `${prompt.id}:${prompt.title}:${prompt.purpose || ''}:${prompt.style || ''}`
  let hash = 0
  for (let index = 0; index < key.length; index += 1) {
    hash = (hash * 31 + key.charCodeAt(index)) >>> 0
  }
  return `tone-${hash % 10}`
}

export function promptCoverKicker(prompt: PromptSummary): string {
  return purposeLabels[prompt.purpose || ''] || prompt.purpose || prompt.recommended_models[0] || '精选提示词'
}

export function promptCoverBadge(prompt: PromptSummary): string {
  return styleLabels[prompt.style || ''] || prompt.style || prompt.recommended_sizes[0] || '极速蹬'
}
