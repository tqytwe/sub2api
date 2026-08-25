import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../AccountsView.vue'),
  'utf8',
)

const dialogs = [
  'CreateAccountModal',
  'EditAccountModal',
  'BulkEditAccountModal',
  'SyncFromCrsModal',
  'ImportDataModal',
  'ReAuthAccountModal',
  'AccountTestModal',
  'AccountStatsModal',
  'ScheduledTestsPanel',
  'TempUnschedStatusModal',
  'ErrorPassthroughRulesModal',
  'TLSFingerprintProfilesModal',
]

describe('AccountsView lazy dialogs', () => {
  it('loads heavy dialogs through defineAsyncComponent', () => {
    expect(source).toContain('defineAsyncComponent')
    for (const dialog of dialogs) {
      expect(source).toMatch(new RegExp(`const ${dialog} = defineAsyncComponent`))
      expect(source).not.toMatch(new RegExp(`import ${dialog} from`))
    }
  })

  it('does not mount closed heavy dialogs', () => {
    for (const dialog of dialogs) {
      expect(source).toMatch(new RegExp(`<${dialog}\\s+v-if=`))
    }
  })
})
