type ChunkRecoveryStorage = Pick<Storage, 'getItem' | 'setItem'>
type ChunkRecoveryLocation = Pick<Location, 'replace'>

interface ChunkRecoveryOptions {
  storage?: ChunkRecoveryStorage
  location?: ChunkRecoveryLocation
  now?: () => number
}

const retryKeyPrefix = 'chunk_reload_attempted:'
// design-governance-allow: visual-evidence - behavior-only chunk recovery; no rendered UI changes.
let globalChunkRecoveryInstalled = false

export function isChunkLoadError(error: unknown): boolean {
  const candidate = error as { message?: string; name?: string } | null
  const message =
    typeof error === 'string'
      ? error
      : candidate?.message ?? ''

  return (
    message.includes('Failed to fetch dynamically imported module') ||
    message.includes('Loading chunk') ||
    message.includes('Loading CSS chunk') ||
    candidate?.name === 'ChunkLoadError'
  )
}

export function recoverFromChunkLoadError(
  error: unknown,
  targetFullPath: string | undefined,
  options: ChunkRecoveryOptions = {}
): boolean {
  if (!isChunkLoadError(error)) {
    return false
  }

  const target = targetFullPath || currentFullPath()
  const storage = options.storage ?? window.sessionStorage
  const location = options.location ?? window.location
  const now = options.now ?? Date.now
  const retryKey = retryKeyPrefix + encodeURIComponent(target)

  if (storage.getItem(retryKey)) {
    console.error('Chunk load error persists after retry. Please clear browser cache.')
    return true
  }

  storage.setItem(retryKey, String(now()))
  console.warn('Chunk load error detected, retrying target route with latest assets...')
  location.replace(target)
  return true
}

export function installGlobalChunkLoadRecovery(): void {
  if (globalChunkRecoveryInstalled || typeof window === 'undefined') {
    return
  }
  globalChunkRecoveryInstalled = true

  window.addEventListener('unhandledrejection', (event) => {
    if (recoverFromChunkLoadError(event.reason, currentFullPath())) {
      event.preventDefault()
    }
  })

  window.addEventListener('error', (event) => {
    const error = event.error || event.message
    if (recoverFromChunkLoadError(error, currentFullPath())) {
      event.preventDefault()
    }
  })
}

function currentFullPath(): string {
  return `${window.location.pathname}${window.location.search}${window.location.hash}`
}
