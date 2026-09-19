import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(process.cwd(), 'src/components/account/CreateAccountModal.vue'),
  'utf8'
)

describe('CreateAccountModal OpenCode account setup', () => {
  it('keeps the OpenCode platform and Zen/GO controls in the create flow', () => {
    expect(source).toContain('@click="selectOpenCodeGoPlatform()"')
    expect(source).toContain('<PlatformIcon platform="opencode_go" size="sm" />')
    expect(source).toContain("@click=\"openCodeAccountMode = 'zen'\"")
    expect(source).toContain("@click=\"openCodeAccountMode = 'go'\"")
    expect(source).toContain("form.platform = 'opencode_go'")
    expect(source).toContain("form.type = 'apikey'")
  })

  it('keeps adaptive endpoints and protocol rules in the create payload', () => {
    expect(source).toContain("defaultCNAdaptiveBaseUrls('opencode_go', mode)")
    expect(source).toContain("v-if=\"isOpenCodeGoPlatform && apiProtocol === 'adaptive'\"")
    expect(source).toContain('v-model:rows="openCodeGoProtocolRules"')
    expect(source).toContain(':plan="openCodeAccountMode"')
    expect(source).toContain('applyOpenCodeGoProtocolRules(credentials, openCodeGoProtocolRules.value, \'create\')')
  })
})
