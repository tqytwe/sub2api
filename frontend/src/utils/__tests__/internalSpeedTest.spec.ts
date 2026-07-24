import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  consumeInternalSpeedTestPayload,
  createOpenAIStreamParser,
  fetchSpeedTestModels,
  normalizeSpeedTestBaseUrl,
  runSpeedTestChatCompletion,
  saveInternalSpeedTestPayload,
} from '@/utils/internalSpeedTest'

function streamFromChunks(chunks: string[]): ReadableStream<Uint8Array> {
  return new ReadableStream<Uint8Array>({
    start(controller) {
      const encoder = new TextEncoder()
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk))
      }
      controller.close()
    },
  })
}

describe('internal speed test utilities', () => {
  beforeEach(() => {
    sessionStorage.clear()
    vi.unstubAllGlobals()
  })

  it.each([
    ['https://api.example.com', 'https://api.example.com/v1'],
    ['https://api.example.com/', 'https://api.example.com/v1'],
    ['https://api.example.com/v1', 'https://api.example.com/v1'],
    ['/v1', '/v1'],
    ['', 'http://localhost:3000/v1'],
  ])('normalizes speed-test base URL %s', (input, expected) => {
    expect(normalizeSpeedTestBaseUrl(input, 'http://localhost:3000')).toBe(expected)
  })

  it('stores and consumes a one-time launch payload without putting the key in a URL', () => {
    saveInternalSpeedTestPayload({
      apiKey: 'sk-local-only',
      baseUrl: 'https://api.example.com',
      keyName: 'Primary key',
    })

    expect(window.location.href).not.toContain('sk-local-only')

    const payload = consumeInternalSpeedTestPayload()
    expect(payload).toMatchObject({
      apiKey: 'sk-local-only',
      baseUrl: 'https://api.example.com',
      keyName: 'Primary key',
    })
    expect(consumeInternalSpeedTestPayload()).toBeNull()
  })

  it('rejects malformed and stale launch payloads', () => {
    sessionStorage.setItem('sub2api_internal_speed_test', '{')
    expect(consumeInternalSpeedTestPayload()).toBeNull()

    sessionStorage.setItem('sub2api_internal_speed_test', JSON.stringify({
      apiKey: 'sk-stale',
      baseUrl: 'https://api.example.com',
      createdAt: Date.now() - 16 * 60 * 1000,
    }))
    expect(consumeInternalSpeedTestPayload()).toBeNull()
  })

  it('buffers split OpenAI SSE lines before parsing JSON', () => {
    const parser = createOpenAIStreamParser()
    const events = [
      ...parser.push('data: {"choices":[{"delta":{"content":"hel'),
      ...parser.push('lo"}}]}\n\ndata: {"choices":[{"delta":{"content":" world"}}]}\n'),
      ...parser.push('\ndata: [DONE]\n\n'),
    ]

    expect(events).toEqual([
      { type: 'delta', content: 'hello' },
      { type: 'delta', content: ' world' },
      { type: 'done' },
    ])
  })

  it('loads models with the selected API key only in the Authorization header', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: [
          { id: 'gpt-5.5', object: 'model' },
          { id: 'claude-fable-5', object: 'model' },
        ],
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const models = await fetchSpeedTestModels({
      apiKey: 'sk-model-list',
      baseUrl: 'https://api.example.com',
    })

    expect(models.map((model) => model.id)).toEqual(['gpt-5.5', 'claude-fable-5'])
    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.example.com/v1/models',
      expect.objectContaining({
        headers: { Authorization: 'Bearer sk-model-list' },
      }),
    )
    expect(fetchMock.mock.calls[0][0]).not.toContain('sk-model-list')
  })

  it('runs a streamed chat speed test across split chunks', async () => {
    vi.spyOn(performance, 'now')
      .mockReturnValueOnce(0)
      .mockReturnValueOnce(250)
      .mockReturnValueOnce(1250)

    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      body: streamFromChunks([
        'data: {"choices":[{"delta":{"content":"fast ' ,
        'response"}}]}\n\n',
        'data: [DONE]\n\n',
      ]),
      headers: new Headers(),
      status: 200,
      statusText: 'OK',
    })
    vi.stubGlobal('fetch', fetchMock)

    const deltas: string[] = []
    const result = await runSpeedTestChatCompletion({
      apiKey: 'sk-chat',
      baseUrl: 'https://api.example.com/v1',
      model: 'gpt-5.5',
      prompt: 'Say hello',
      signal: new AbortController().signal,
      onDelta: (text) => deltas.push(text),
    })

    expect(deltas).toEqual(['fast response'])
    expect(result.outputText).toBe('fast response')
    expect(result.firstTokenLatencyMs).toBe(250)
    expect(result.totalMs).toBe(1250)
    expect(result.tokensPerSecond).toBeGreaterThan(0)
    expect(fetchMock.mock.calls[0][0]).toBe('https://api.example.com/v1/chat/completions')
    expect(fetchMock.mock.calls[0][0]).not.toContain('sk-chat')
  })
})
