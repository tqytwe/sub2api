export const INTERNAL_SPEED_TEST_STORAGE_KEY = 'sub2api_internal_speed_test'

const PAYLOAD_MAX_AGE_MS = 15 * 60 * 1000

export interface InternalSpeedTestPayload {
  apiKey: string
  baseUrl: string
  keyName?: string
  createdAt: number
}

export interface InternalSpeedTestSaveInput {
  apiKey: string
  baseUrl: string
  keyName?: string
}

export interface SpeedTestModel {
  id: string
  object?: string
  owned_by?: string
}

export type OpenAIStreamParserEvent =
  | { type: 'delta'; content: string }
  | { type: 'usage'; completionTokens?: number }
  | { type: 'done' }

export interface SpeedTestChatCompletionInput {
  apiKey: string
  baseUrl: string
  model: string
  prompt: string
  signal?: AbortSignal
  onDelta?: (content: string) => void
}

export interface SpeedTestChatCompletionResult {
  outputText: string
  outputTokens: number
  firstTokenLatencyMs: number | null
  totalMs: number
  tokensPerSecond: number
}

export function normalizeSpeedTestBaseUrl(baseUrl: string, fallbackOrigin?: string): string {
  const origin =
    fallbackOrigin ??
    (typeof window !== 'undefined' && window.location?.origin ? window.location.origin : '')
  const raw = (baseUrl || origin).trim().replace(/\/+$/, '')
  if (!raw) return '/v1'
  return `${raw.replace(/\/v1$/i, '')}/v1`
}

function speedTestEndpoint(baseUrl: string, path: string): string {
  const base = normalizeSpeedTestBaseUrl(baseUrl)
  const suffix = path.replace(/^\/+/, '').replace(/^v1\/?/i, '')
  return suffix ? `${base}/${suffix}` : base
}

function authHeaders(apiKey: string, extra?: HeadersInit): HeadersInit {
  return {
    Authorization: `Bearer ${apiKey.trim()}`,
    ...extra,
  }
}

export function saveInternalSpeedTestPayload(input: InternalSpeedTestSaveInput): void {
  const apiKey = input.apiKey.trim()
  const baseUrl = input.baseUrl.trim()
  if (!apiKey || !baseUrl || typeof sessionStorage === 'undefined') return

  const payload: InternalSpeedTestPayload = {
    apiKey,
    baseUrl,
    keyName: input.keyName?.trim() || undefined,
    createdAt: Date.now(),
  }
  sessionStorage.setItem(INTERNAL_SPEED_TEST_STORAGE_KEY, JSON.stringify(payload))
}

export function consumeInternalSpeedTestPayload(maxAgeMs = PAYLOAD_MAX_AGE_MS): InternalSpeedTestPayload | null {
  if (typeof sessionStorage === 'undefined') return null

  const raw = sessionStorage.getItem(INTERNAL_SPEED_TEST_STORAGE_KEY)
  sessionStorage.removeItem(INTERNAL_SPEED_TEST_STORAGE_KEY)
  if (!raw) return null

  try {
    const parsed = JSON.parse(raw) as Partial<InternalSpeedTestPayload>
    if (
      typeof parsed.apiKey !== 'string' ||
      !parsed.apiKey.trim() ||
      typeof parsed.baseUrl !== 'string' ||
      !parsed.baseUrl.trim() ||
      typeof parsed.createdAt !== 'number'
    ) {
      return null
    }
    if (Date.now() - parsed.createdAt > maxAgeMs) return null

    return {
      apiKey: parsed.apiKey,
      baseUrl: parsed.baseUrl,
      keyName: typeof parsed.keyName === 'string' && parsed.keyName.trim() ? parsed.keyName : undefined,
      createdAt: parsed.createdAt,
    }
  } catch {
    return null
  }
}

export function createOpenAIStreamParser() {
  let buffer = ''

  function parseData(data: string): OpenAIStreamParserEvent[] {
    const trimmed = data.trim()
    if (!trimmed) return []
    if (trimmed === '[DONE]') return [{ type: 'done' }]

    const parsed = JSON.parse(trimmed) as {
      choices?: Array<{
        delta?: { content?: unknown }
        message?: { content?: unknown }
      }>
      usage?: { completion_tokens?: unknown }
    }
    const events: OpenAIStreamParserEvent[] = []
    const content = (parsed.choices ?? [])
      .map((choice) => {
        const deltaContent = choice.delta?.content
        if (typeof deltaContent === 'string') return deltaContent
        const messageContent = choice.message?.content
        return typeof messageContent === 'string' ? messageContent : ''
      })
      .join('')
    if (content) events.push({ type: 'delta', content })

    const completionTokens = parsed.usage?.completion_tokens
    if (typeof completionTokens === 'number') {
      events.push({ type: 'usage', completionTokens })
    }
    return events
  }

  function parseLine(line: string): OpenAIStreamParserEvent[] {
    const trimmed = line.replace(/\r$/, '').trim()
    if (!trimmed || trimmed.startsWith(':') || trimmed.startsWith('event:')) return []
    if (trimmed.startsWith('data:')) return parseData(trimmed.slice(5))
    if (trimmed.startsWith('{')) return parseData(trimmed)
    return []
  }

  return {
    push(chunk: string): OpenAIStreamParserEvent[] {
      buffer += chunk
      const lines = buffer.split('\n')
      buffer = lines.pop() ?? ''
      return lines.flatMap(parseLine)
    },
    flush(): OpenAIStreamParserEvent[] {
      const remaining = buffer
      buffer = ''
      return remaining ? parseLine(remaining) : []
    },
  }
}

async function parseSpeedTestError(response: Response): Promise<Error> {
  try {
    const body = await response.json()
    const message = body?.error?.message || body?.message || response.statusText || `HTTP ${response.status}`
    const error = new Error(message)
    ;(error as Error & { status?: number }).status = response.status
    return error
  } catch {
    const error = new Error(response.statusText || `HTTP ${response.status}`)
    ;(error as Error & { status?: number }).status = response.status
    return error
  }
}

export async function fetchSpeedTestModels(input: {
  apiKey: string
  baseUrl: string
  signal?: AbortSignal
}): Promise<SpeedTestModel[]> {
  const response = await fetch(speedTestEndpoint(input.baseUrl, '/v1/models'), {
    headers: authHeaders(input.apiKey),
    signal: input.signal,
  })
  if (!response.ok) throw await parseSpeedTestError(response)
  const body = await response.json() as { data?: Array<Partial<SpeedTestModel>> }
  const seen = new Set<string>()
  return (body.data ?? [])
    .filter((model): model is SpeedTestModel => typeof model.id === 'string' && model.id.trim().length > 0)
    .filter((model) => {
      if (seen.has(model.id)) return false
      seen.add(model.id)
      return true
    })
}

function estimateOutputTokens(content: string): number {
  const trimmed = content.trim()
  if (!trimmed) return 0
  const words = trimmed.split(/\s+/).filter(Boolean).length
  const charEstimate = Math.ceil(trimmed.length / 4)
  return Math.max(1, words, charEstimate)
}

export async function runSpeedTestChatCompletion(
  input: SpeedTestChatCompletionInput,
): Promise<SpeedTestChatCompletionResult> {
  const startedAt = performance.now()
  const response = await fetch(speedTestEndpoint(input.baseUrl, '/v1/chat/completions'), {
    method: 'POST',
    headers: authHeaders(input.apiKey, {
      'Content-Type': 'application/json',
    }),
    body: JSON.stringify({
      model: input.model,
      messages: [
        {
          role: 'user',
          content: input.prompt,
        },
      ],
      stream: true,
      temperature: 0,
      max_tokens: 256,
    }),
    signal: input.signal,
  })

  if (!response.ok) throw await parseSpeedTestError(response)
  const reader = response.body?.getReader()
  if (!reader) throw new Error('No response body')

  const decoder = new TextDecoder()
  const parser = createOpenAIStreamParser()
  let outputText = ''
  let completionTokens: number | undefined
  let firstTokenLatencyMs: number | null = null

  function handleEvents(events: OpenAIStreamParserEvent[]) {
    for (const event of events) {
      if (event.type === 'delta') {
        if (firstTokenLatencyMs === null) {
          firstTokenLatencyMs = performance.now() - startedAt
        }
        outputText += event.content
        input.onDelta?.(event.content)
      } else if (event.type === 'usage') {
        completionTokens = event.completionTokens
      }
    }
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    handleEvents(parser.push(decoder.decode(value, { stream: true })))
  }
  handleEvents(parser.push(decoder.decode()))
  handleEvents(parser.flush())

  const totalMs = performance.now() - startedAt
  const outputTokens = completionTokens ?? estimateOutputTokens(outputText)
  const activeMs = firstTokenLatencyMs === null ? totalMs : Math.max(1, totalMs - firstTokenLatencyMs)

  return {
    outputText,
    outputTokens,
    firstTokenLatencyMs,
    totalMs,
    tokensPerSecond: outputTokens > 0 ? outputTokens / (activeMs / 1000) : 0,
  }
}
