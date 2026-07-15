/** Lightweight growth analytics — forwards to window.dataLayer when present. */
export type GrowthEventName =
  | 'image_studio_workspace_view'
  | 'image_studio_intent_select'
  | 'image_studio_template_select'
  | 'image_studio_generate_click'
  | 'image_studio_generate_success'
  | 'image_studio_generate_fail'
  | 'image_studio_result_view'
  | 'image_studio_insufficient_balance'
  | 'image_studio_download_click'
  | 'image_studio_download_success'
  | 'image_studio_download_fail'
  | 'image_studio_size_change'
  | 'image_studio_regenerate_same'
  | 'farm_quest_complete'
  | 'farm_daily_tab_view'
  | 'play_hub_action_click'

export function trackGrowthEvent(name: GrowthEventName, props?: Record<string, unknown>) {
  const payload = { event: name, ...props, ts: Date.now() }
  if (typeof window !== 'undefined') {
    const w = window as Window & { dataLayer?: unknown[] }
    w.dataLayer = w.dataLayer || []
    w.dataLayer.push(payload)
  }
  if (import.meta.env.DEV) {
    console.debug('[growth]', payload)
  }
}

const STUDIO_FIRST_WIN_KEY = 'studio_first_win'
const STUDIO_LAST_TEMPLATE_KEY = 'image_studio_last_template'

export function hasStudioFirstWin(): boolean {
  return localStorage.getItem(STUDIO_FIRST_WIN_KEY) === '1'
}

export function markStudioFirstWin() {
  localStorage.setItem(STUDIO_FIRST_WIN_KEY, '1')
}

export function getStudioLastTemplate(): string | null {
  return localStorage.getItem(STUDIO_LAST_TEMPLATE_KEY)
}

export function setStudioLastTemplate(templateId: string) {
  localStorage.setItem(STUDIO_LAST_TEMPLATE_KEY, templateId)
}

export function getStudioAutoCleanup(): boolean {
  const v = localStorage.getItem('image_studio_auto_cleanup')
  return v !== '0'
}

export function setStudioAutoCleanup(enabled: boolean) {
  localStorage.setItem('image_studio_auto_cleanup', enabled ? '1' : '0')
}

const QUEST_TRACKED_PREFIX = 'farm_quest_tracked_'

/** Fire farm_quest_complete at most once per quest per local day. */
export function trackQuestCompleteOnce(questKey: string) {
  const day = new Date().toISOString().slice(0, 10)
  const key = `${QUEST_TRACKED_PREFIX}${questKey}_${day}`
  if (localStorage.getItem(key) === '1') return
  localStorage.setItem(key, '1')
  trackGrowthEvent('farm_quest_complete', { quest_key: questKey })
}
