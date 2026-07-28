import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const root = `${process.cwd()}/src/views/public`
const playViews = ['ArenaView.vue', 'AgentTeamView.vue', 'BlindboxView.vue', 'QuizQuestView.vue']

describe('authenticated play shell integration', () => {
  it('uses the shared authenticated shell for every playable module', () => {
    for (const view of playViews) {
      const source = readFileSync(`${root}/${view}`, 'utf8')

      expect(source).toContain("AuthenticatedPlayShell")
      expect(source).toContain('v-if="!authStore.isAuthenticated"')
    }
  })
})
