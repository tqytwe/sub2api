import type { PromptSummary } from '@/api/prompts'

const genericTemplatePaths = new Set([
  '/image-studio/templates/ecom-white-bg.webp',
  '/image-studio/templates/free-create.webp',
  '/image-studio/templates/xhs-cover.webp',
])

const purposeMessageKeys: Record<string, string> = {
  'ad-creative': 'promptLibrary.cover.purpose.adCreative',
  'app-web-design': 'promptLibrary.cover.purpose.appWebDesign',
  'document-presentation': 'promptLibrary.cover.purpose.documentPresentation',
  'ecommerce-main-image': 'promptLibrary.cover.purpose.ecommerceMainImage',
  'free-creation': 'promptLibrary.cover.purpose.freeCreation',
  'game-asset': 'promptLibrary.cover.purpose.gameAsset',
  'infographic-edu-visual': 'promptLibrary.cover.purpose.infographicEduVisual',
  'live-commerce': 'promptLibrary.cover.purpose.liveCommerce',
  'packaging-design': 'promptLibrary.cover.purpose.packagingDesign',
  'poster-typography': 'promptLibrary.cover.purpose.posterTypography',
  'profile-avatar': 'promptLibrary.cover.purpose.profileAvatar',
  'social-media-post': 'promptLibrary.cover.purpose.socialMediaPost',
  'virtual-try-on': 'promptLibrary.cover.purpose.virtualTryOn',
  'youtube-thumbnail': 'promptLibrary.cover.purpose.youtubeThumbnail',
}

const styleMessageKeys: Record<string, string> = {
  '3d-render': 'promptLibrary.cover.style.render3d',
  'cinematic-film-still': 'promptLibrary.cover.style.cinematicFilmStill',
  'flat-vector': 'promptLibrary.cover.style.flatVector',
  'luxury-editorial': 'promptLibrary.cover.style.luxuryEditorial',
  'minimal-clean': 'promptLibrary.cover.style.minimalClean',
  photorealistic: 'promptLibrary.cover.style.photorealistic',
  'poster-typography': 'promptLibrary.cover.style.posterTypography',
  watercolor: 'promptLibrary.cover.style.watercolor',
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

export function promptCoverKickerMessageKey(prompt: PromptSummary): string | undefined {
  return purposeMessageKeys[prompt.purpose || '']
}

export function promptCoverKickerFallback(prompt: PromptSummary): string {
  return prompt.purpose || prompt.recommended_models[0] || ''
}

export function promptCoverBadgeMessageKey(prompt: PromptSummary): string | undefined {
  return styleMessageKeys[prompt.style || '']
}

export function promptCoverBadgeFallback(prompt: PromptSummary): string {
  return prompt.style || prompt.recommended_sizes[0] || ''
}
