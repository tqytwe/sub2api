/** Odometer column model — supports arbitrary digit counts with tail spin. */

export const ODOMETER_STRIP_LEN = 50
export const ODOMETER_LAND_BASE = 20
export const ODOMETER_DEFAULT_SPIN_TAIL = 3

export type OdometerStaticColumn = {
  key: string
  type: 'static'
  ch: string
}

export type OdometerDigitColumnDef = {
  key: string
  type: 'digit'
  targetDigit: number
  isTail: boolean
  tailRank: number
}

export type OdometerColumnDef = OdometerStaticColumn | OdometerDigitColumnDef

export type OdometerDigitState = OdometerDigitColumnDef & {
  offset: number
  animating: boolean
  durationMs: number
  delayMs: number
}

export type OdometerColumnState = OdometerStaticColumn | OdometerDigitState

export function visibleDigit(offset: number): number {
  return ((offset % 10) + 10) % 10
}

export function landOffset(digit: number): number {
  return ODOMETER_LAND_BASE + digit
}

/** Next offset: always scroll forward through extra cycles, land exactly on `toDigit`. */
export function spinOffset(fromOffset: number, toDigit: number, extraCycles: number): number {
  const current = visibleDigit(fromOffset)
  let step = (toDigit - current + 10) % 10
  if (step === 0) step = 10
  return fromOffset + Math.max(1, extraCycles) * 10 + step
}

export function parseValueColumns(value: string, spinTail = ODOMETER_DEFAULT_SPIN_TAIL): OdometerColumnDef[] {
  const digitPositions: { idx: number; digit: number }[] = []
  for (let i = 0; i < value.length; i++) {
    const ch = value[i]!
    if (/\d/.test(ch)) digitPositions.push({ idx: i, digit: Number(ch) })
  }

  const tailFrom = Math.max(0, digitPositions.length - spinTail)

  return value.split('').map((ch, idx) => {
    if (!/\d/.test(ch)) {
      return { key: `s-${idx}`, type: 'static' as const, ch }
    }
    const pos = digitPositions.findIndex((d) => d.idx === idx)
    const isTail = pos >= tailFrom
    return {
      key: `d-${idx}`,
      type: 'digit' as const,
      targetDigit: Number(ch),
      isTail,
      tailRank: isTail ? digitPositions.length - 1 - pos : 0,
    }
  })
}

export function createDigitState(def: OdometerDigitColumnDef, offset = 0): OdometerDigitState {
  return {
    ...def,
    offset,
    animating: false,
    durationMs: 0,
    delayMs: 0,
  }
}

export function randomTailSpinPlan(tailRank: number): { extraCycles: number; durationMs: number; delayMs: number } {
  const extraCycles = 1 + Math.floor(Math.random() * 4)
  const durationMs = 650 + Math.floor(Math.random() * 750)
  const delayMs = tailRank * (60 + Math.floor(Math.random() * 90)) + Math.floor(Math.random() * 80)
  return { extraCycles, durationMs, delayMs }
}

export function randomEntrancePlan(index: number): { extraCycles: number; durationMs: number; delayMs: number } {
  const extraCycles = 1 + Math.floor(Math.random() * 3)
  const durationMs = 900 + Math.floor(Math.random() * 600)
  const delayMs = 120 + index * (70 + Math.floor(Math.random() * 50))
  return { extraCycles, durationMs, delayMs }
}

export function syncColumnsFromValue(
  prev: OdometerColumnState[],
  value: string,
  spinTail: number,
  opts: { animateTail: boolean; entrance: boolean },
): OdometerColumnState[] {
  const defs = parseValueColumns(value, spinTail)
  const prevDigit = new Map<string, OdometerDigitState>()
  for (const col of prev) {
    if (col.type === 'digit') prevDigit.set(col.key, col)
  }

  let digitIndex = 0
  return defs.map((def) => {
    if (def.type === 'static') return def

    const old = prevDigit.get(def.key)
    const startOffset = old?.offset ?? 0
    const col = createDigitState(def, startOffset)

    if (opts.entrance) {
      const plan = randomEntrancePlan(digitIndex++)
      col.animating = true
      col.durationMs = plan.durationMs
      col.delayMs = plan.delayMs
      col.offset = spinOffset(0, def.targetDigit, plan.extraCycles)
      return col
    }

    if (!def.isTail || !opts.animateTail) {
      col.offset = landOffset(def.targetDigit)
      col.animating = false
      return col
    }

    const prevTarget = old?.targetDigit
    if (
      prevTarget === def.targetDigit &&
      old &&
      visibleDigit(old.offset) === def.targetDigit &&
      !old.animating
    ) {
      col.offset = landOffset(def.targetDigit)
      return col
    }

    if (old?.animating) {
      col.offset = landOffset(def.targetDigit)
      col.animating = false
      return col
    }

    const plan = randomTailSpinPlan(def.tailRank)
    col.offset = spinOffset(startOffset, def.targetDigit, plan.extraCycles)
    col.animating = true
    col.durationMs = plan.durationMs
    col.delayMs = plan.delayMs
    return col
  })
}
