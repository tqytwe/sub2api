export type PlayFeatureId = 'blindbox' | 'arena' | 'quiz-quest' | 'agent-team'

export interface PlayFeatureMeta {
  id: PlayFeatureId
  route: string
  titleKey: string
}

export const PLAY_FEATURES: PlayFeatureMeta[] = [
  { id: 'blindbox', route: '/blindbox', titleKey: 'play.blindbox.title' },
  { id: 'arena', route: '/arena', titleKey: 'play.arena.title' },
  { id: 'quiz-quest', route: '/quiz-quest', titleKey: 'play.quizQuest.title' },
  { id: 'agent-team', route: '/agent-team', titleKey: 'play.agentTeam.title' },
]

export function findPlayFeature(id: string): PlayFeatureMeta | undefined {
  return PLAY_FEATURES.find((f) => f.id === id)
}

/** Home ChannelTV channel index → destination */
export const CHANNEL_DESTINATIONS = [
  { route: '/blindbox' },
  { route: '/check-in' },
  { route: '/agent-team' },
  { route: '/arena' },
] as const

export type HomeChannelId = 'blindbox' | 'check-in' | 'agent-team' | 'arena'

export interface HomeChannelMeta {
  key: `ch${1 | 2 | 3 | 4}`
  id: HomeChannelId
  settingKey: 'play_blindbox_enabled' | 'play_checkin_enabled' | 'play_agent_team_enabled' | 'play_arena_enabled'
  route: string
}

export const HOME_CHANNELS: HomeChannelMeta[] = [
  { key: 'ch1', id: 'blindbox', settingKey: 'play_blindbox_enabled', route: CHANNEL_DESTINATIONS[0].route },
  { key: 'ch2', id: 'check-in', settingKey: 'play_checkin_enabled', route: CHANNEL_DESTINATIONS[1].route },
  { key: 'ch3', id: 'agent-team', settingKey: 'play_agent_team_enabled', route: CHANNEL_DESTINATIONS[2].route },
  { key: 'ch4', id: 'arena', settingKey: 'play_arena_enabled', route: CHANNEL_DESTINATIONS[3].route },
]

export function enabledHomeChannels(settings: Partial<Record<HomeChannelMeta['settingKey'], boolean>> | null | undefined): HomeChannelMeta[] {
  if (!settings) return []
  return HOME_CHANNELS.filter((channel) => settings[channel.settingKey] === true)
}
