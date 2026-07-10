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
