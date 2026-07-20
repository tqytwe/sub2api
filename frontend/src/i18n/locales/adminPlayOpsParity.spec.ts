import { describe, expect, it } from 'vitest'
import zh from './zh'
import en from './en'

function collectKeys(value: unknown, prefix = ''): string[] {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return [prefix]
  return Object.entries(value as Record<string, unknown>)
    .flatMap(([key, child]) => collectKeys(child, prefix ? `${prefix}.${key}` : key))
}

describe('admin Play Ops locale parity', () => {
  it('keeps every runtime Chinese and English Play Ops key symmetric', () => {
    const zhPlayOps = (zh.admin as Record<string, unknown>).playOps
    const enPlayOps = (en.admin as Record<string, unknown>).playOps
    expect(collectKeys(zhPlayOps).sort()).toEqual(collectKeys(enPlayOps).sort())
  })

  it('loads the complete member repair and event namespaces through the runtime locale entrypoints', () => {
    const required = [
      'admin.playOps.addMember',
      'admin.playOps.memberRepair.search',
      'admin.playOps.memberRepair.operationAdd',
      'admin.playOps.memberRepair.operationMove',
      'admin.playOps.memberRepair.reason',
      'admin.playOps.memberRepair.confirm',
      'admin.playOps.memberRepair.blockers.PLAY_TEAM_MEMBER_MOVE_REQUIRED',
      'admin.playOps.memberRepair.warnings.PLAY_TEAM_SOURCE_WILL_ARCHIVE',
      'admin.playOps.events.admin_member_added',
      'admin.playOps.events.admin_member_moved',
      'admin.playOps.events.team_archived',
      'admin.playOps.settlementStatus.completed',
      'admin.playOps.payoutStatus.paid',
    ]
    const zhKeys = new Set(collectKeys(zh))
    const enKeys = new Set(collectKeys(en))
    for (const key of required) {
      expect(zhKeys.has(key), `missing zh key: ${key}`).toBe(true)
      expect(enKeys.has(key), `missing en key: ${key}`).toBe(true)
    }
  })

  it('keeps the Chinese Play Ops interface free of English operational labels', () => {
    const zhPlayOps = (zh.admin as Record<string, unknown>).playOps
    expect(JSON.stringify(zhPlayOps)).not.toMatch(/\b(?:Token|Tokens|Team|Arena)\b/)
  })
})
