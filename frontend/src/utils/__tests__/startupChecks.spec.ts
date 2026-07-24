import { describe, expect, it } from 'vitest'
import { shouldProbeSetupStatusOnStartup } from '@/utils/startupChecks'

describe('startup setup checks', () => {
  it('skips the setup probe when public settings were injected into the HTML', () => {
    expect(shouldProbeSetupStatusOnStartup('/login', true)).toBe(false)
    expect(shouldProbeSetupStatusOnStartup('/', true)).toBe(false)
  })

  it('keeps the setup probe for static or dev fallbacks without injected settings', () => {
    expect(shouldProbeSetupStatusOnStartup('/login', false)).toBe(true)
    expect(shouldProbeSetupStatusOnStartup('/', false)).toBe(true)
  })

  it('lets the setup route own its own setup-status check', () => {
    expect(shouldProbeSetupStatusOnStartup('/setup', false)).toBe(false)
    expect(shouldProbeSetupStatusOnStartup('/setup/database', false)).toBe(false)
  })
})
