import axios, { type AxiosAdapter } from 'axios'

// Gate only the transport observation under test. The page, auth store, Axios
// interceptors and native HTTP adapter continue to run normally.
export function holdHttpBoundary(stage: string, boundary: 'abort' | 'arrival') {
  let release!: () => void
  const released = new Promise<void>(resolve => { release = resolve })
  const state = { reached: false }
  const http = axios.getAdapter('http')
  const adapter: AxiosAdapter = async config => {
    const url = new URL(axios.getUri(config))
    const actualStage = url.searchParams.get('section') || (url.searchParams.has('view') ? 'summary' : 'list')
    if (!url.pathname.includes('/requests') || actualStage !== stage) return http(config)
    if (boundary === 'arrival') {
      state.reached = true
      await released
      return http(config)
    }
    const signal = config.signal, transport = new AbortController()
    const onAbort = () => {
      state.reached = true
      void released.then(() => transport.abort())
    }
    if (signal?.aborted) onAbort()
    else signal?.addEventListener?.('abort', onAbort, { once: true })
    try { return await http({ ...config, signal: transport.signal }) }
    finally { signal?.removeEventListener?.('abort', onAbort) }
  }
  return { adapter, state, release }
}
