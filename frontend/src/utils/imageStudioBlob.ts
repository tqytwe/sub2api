export function extensionForContentType(contentType?: string): string {
  const ct = String(contentType || '').toLowerCase()
  if (ct.includes('jpeg') || ct.includes('jpg')) return '.jpg'
  if (ct.includes('webp')) return '.webp'
  if (ct.includes('gif')) return '.gif'
  return '.png'
}

export function filenameForAsset(jobId: string, index: number, contentType?: string) {
  return `image-studio-${jobId.slice(0, 8)}-${index + 1}${extensionForContentType(contentType)}`
}

export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

export function isExternalAssetUrl(raw: string): boolean {
  return (
    raw.startsWith('http://') ||
    raw.startsWith('https://') ||
    raw.startsWith('data:') ||
    raw.startsWith('blob:')
  )
}

export function isStudioAssetApiPath(raw: string): boolean {
  return raw.includes('/image-studio/assets/')
}
