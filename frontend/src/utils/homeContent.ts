export function isHomeContentUrl(content: string): boolean {
  return /^https?:\/\//.test(content.trim())
}

export async function sanitizeHomeContent(content: string): Promise<string> {
  if (!content.trim() || isHomeContentUrl(content)) {
    return ''
  }

  const { default: DOMPurify } = await import('dompurify')
  return DOMPurify.sanitize(content)
}
