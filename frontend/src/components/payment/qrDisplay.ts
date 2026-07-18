const HOSTED_QR_HOSTS = [
  'xunhupay.com',
  'dpweixin.com',
  'diypc.com.cn',
]

const IMAGE_EXTENSIONS = /\.(?:png|jpe?g|gif|webp|svg)(?:$|[?#])/i

export function getHostedQRCodeImageUrl(qrCode: string): string {
  const value = qrCode.trim()
  if (!value) return ''
  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    return ''
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return ''
  }
  const host = parsed.hostname.toLowerCase()
  if (HOSTED_QR_HOSTS.some(domain => host === domain || host.endsWith(`.${domain}`))) {
    return value
  }
  if (IMAGE_EXTENSIONS.test(parsed.pathname)) {
    return value
  }
  return ''
}
