import { describe, expect, it } from 'vitest'

import { enabledHomeChannels } from '../play-features'

describe('homepage channel visibility', () => {
  it('fails closed until a public feature switch is explicitly enabled', () => {
    expect(enabledHomeChannels(null)).toEqual([])
    expect(enabledHomeChannels({
      play_blindbox_enabled: false,
      play_checkin_enabled: false,
      play_agent_team_enabled: false,
      play_arena_enabled: false,
    })).toEqual([])
  })

  it('keeps enabled channels in their stable route order', () => {
    expect(enabledHomeChannels({
      play_blindbox_enabled: true,
      play_checkin_enabled: false,
      play_agent_team_enabled: true,
      play_arena_enabled: false,
    }).map((channel) => ({ id: channel.id, route: channel.route }))).toEqual([
      { id: 'blindbox', route: '/blindbox' },
      { id: 'agent-team', route: '/agent-team' },
    ])
  })
})
