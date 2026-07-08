/** Odometer helpers — stable prefix text + tail digit rollers. */

export const ODOMETER_STRIP_LEN = 50
export const ODOMETER_LAND_BASE = 20
export const ODOMETER_DEFAULT_SPIN_TAIL = 3

export type TailDigitState = {
  key: string
  targetDigit: number
  tailRank: number
  offset: number
  animating: boolean
  durationMs: number
  delayMs: number
}

export function visibleDigit(offset: number): number {
  return ((offset % 10) + 10) % 10
}

export function landOffset(digit: number): number {
  return ODOMETER_LAND_BASE + digit
}

export function spinOffset(fromOffset: number, toDigit: number, extraCycles: number): number {
  const current = visibleDigit(fromOffset)
  let step = (toDigit - current + 10) % 10
  if (step === 0) step = 10
  return fromOffset + Math.max(1, extraCycles) * 10 + step
}

export function splitStableTail(value: string, spinTail = ODOMETER_DEFAULT_SPIN_TAIL): {
  stable: string
  tails: { key: string; digit: number; tailRank: number }[]
} {
  const digitIndexes: number[] = []
  for (let i = 0; i < value.length; i++) {
    if (/\d/.test(value[i]!)) digitIndexes.push(i)
  }

  if (digitIndexes.length === 0) {
    return { stable: value, tails: [] }
  }

  const tailFrom = Math.max(0, digitIndexes.length - spinTail)
  const tailIndexSet = new Set(digitIndexes.slice(tailFrom))

  const tails: { key: string; digit: number; tailRank: number }[] = []
  let stable = ''

  for (let i = 0; i < value.length; i++) {
    const ch = value[i]!
    if (!tailIndexSet.has(i)) {
      stable += ch
      continue
    }
    const tailRank = digitIndexes.length - 1 - digitIndexes.indexOf(i)
    tails.push({ key: `t-${i}`, digit: Number(ch), tailRank })
  }

  return { stable, tails }
}

export function createTailState(
  def: { key: string; digit: number; tailRank: number },
  offset = landOffset(def.digit),
): TailDigitState {
  return {
    key: def.key,
    targetDigit: def.digit,
    tailRank: def.tailRank,
    offset,
    animating: false,
    durationMs: 0,
    delayMs: 0,
  }
}

export function randomTailSpinPlan(tailRank: number): { extraCycles: number; durationMs: number; delayMs: number } {
  const extraCycles = 1 + Math.floor(Math.random() * 4)
  const durationMs = 700 + Math.floor(Math.random() * 800)
  const delayMs = tailRank * (50 + Math.floor(Math.random() * 80)) + Math.floor(Math.random() * 70)
  return { extraCycles, durationMs, delayMs }
}

export function randomEntrancePlan(index: number): { extraCycles: number; durationMs: number; delayMs: number } {
  const extraCycles = 1 + Math.floor(Math.random() * 3)
  const durationMs = 850 + Math.floor(Math.random() * 550)
  const delayMs = 80 + index * (60 + Math.floor(Math.random() * 40))
  return { extraCycles, durationMs, delayMs }
}

export function syncTailDigits(
  prev: TailDigitState[],
  value: string,
  spinTail: number,
  opts: { animate: boolean; entrance: boolean },
): { stable: string; tails: TailDigitState[] } {
  const { stable, tails: defs } = splitStableTail(value, spinTail)
  const prevMap = new Map(prev.map((t) => [t.key, t]))

  const tails = defs.map((def, index) => {
    const old = prevMap.get(def.key)
    const col = createTailState(def, old?.offset ?? landOffset(def.digit))

    if (opts.entrance && opts.animate) {
      const plan = randomEntrancePlan(index)
      col.offset = 0
      col.animating = true
      col.durationMs = plan.durationMs
      col.delayMs = plan.delayMs
      col.offset = spinOffset(0, def.digit, plan.extraCycles)
      return col
    }

    if (!opts.animate) {
      col.offset = landOffset(def.digit)
      col.animating = false
      return col
    }

    if (
      old &&
      old.targetDigit === def.digit &&
      visibleDigit(old.offset) === def.digit &&
      !old.animating
    ) {
      col.offset = landOffset(def.digit)
      return col
    }

    if (old?.animating) {
      col.offset = landOffset(def.digit)
      col.animating = false
      return col
    }

    const plan = randomTailSpinPlan(def.tailRank)
    col.offset = spinOffset(old?.offset ?? landOffset(def.digit), def.digit, plan.extraCycles)
    col.animating = true
    col.durationMs = plan.durationMs
    col.delayMs = plan.delayMs
    return col
  })

  return { stable, tails }
}
