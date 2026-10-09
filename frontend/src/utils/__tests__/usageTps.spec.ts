import { describe, expect, it } from 'vitest'
import { formatUsageOutputTps, usageOutputTps } from '../usageTps'

describe('per-record output TPS API contract', () => {
  it('uses the server value without subtracting first-token wait or recomputing', () => {
    expect(usageOutputTps({ output_tps: 20 })).toBe(20)
    expect(formatUsageOutputTps({ output_tps: 20 })).toBe('20.0 t/s')
    expect(formatUsageOutputTps({ output_tps: 123.8 })).toBe('124 t/s')
    expect(formatUsageOutputTps({ output_tps: 0.001 })).toBe('<0.1 t/s')
  })
  it.each([undefined, null, NaN, Infinity, -Infinity, -1, 0])('does not invent a zero for %s', (output_tps) => {
    expect(formatUsageOutputTps({ output_tps })).toBeNull()
  })
  it('computes valid old API rows only when the server field is absent', () => {
    const oldRow = { output_tokens: 200, duration_ms: 10000, first_token_ms: 9000 }
    expect(usageOutputTps(oldRow)).toBe(20)
    expect(usageOutputTps({ ...oldRow, output_tps: null })).toBeNull()
    expect(usageOutputTps({ ...oldRow, output_tps: 15 })).toBe(15)
    expect(usageOutputTps({ ...oldRow, output_tps: NaN })).toBeNull()
    expect(usageOutputTps({})).toBeNull()
    expect(formatUsageOutputTps(null)).toBeNull()
  })
  it.each([
    { output_tokens: 1, duration_ms: 1000 },
    { output_tokens: 200, duration_ms: 0 },
    { output_tokens: 200, duration_ms: -1 },
    { output_tokens: 200, duration_ms: Infinity },
    { output_tokens: NaN, duration_ms: 1000 },
    { output_tokens: 200, duration_ms: 1000, image_count: 1 },
    { output_tokens: 200, duration_ms: 1000, image_output_tokens: 20 },
    { output_tokens: 200, duration_ms: 1000, billing_mode: 'image' },
    { output_tokens: 200, duration_ms: 1000, billing_mode: 'video' },
    { output_tokens: 200, duration_ms: 1000, media_type: 'image' },
    { output_tokens: 200, duration_ms: 1000, media_type: 'video' },
    { output_tokens: 200, duration_ms: 1000, video_count: 1 },
  ])('preserves the same unavailable rules for an old API row: %s', (row) => {
    expect(usageOutputTps(row)).toBeNull()
  })
})
