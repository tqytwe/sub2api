/** Public docs category/page ids shared by localized documentation payloads. */
export interface PublicDocPage {
  id: string
}

export interface PublicDocCategory {
  id: string
  categoryKey: string
  pages: PublicDocPage[]
}

export const PUBLIC_DOC_TREE: PublicDocCategory[] = [
  {
    id: 'tutorial',
    categoryKey: 'tutorial',
    pages: [
      { id: 'quick-start' },
      { id: 'api-key' },
      { id: 'concurrency' },
    ],
  },
  {
    id: 'recharge-vip',
    categoryKey: 'recharge-vip',
    pages: [
      { id: 'vip-levels' },
      { id: 'how-to-recharge' },
      { id: 'check-in' },
      { id: 'discount-examples' },
      { id: 'faq' },
      { id: 'blindbox-rewards' },
      { id: 'image-studio' },
      { id: 'token-farm' },
    ],
  },
  {
    id: 'about-us',
    categoryKey: 'about-us',
    pages: [
      { id: 'about-us-overview' },
      { id: 'about-us-privacy' },
    ],
  },
  {
    id: 'model-learning',
    categoryKey: 'model-learning',
    pages: [
      { id: 'model-list' },
      { id: 'choose-model' },
      { id: 'best-practices' },
    ],
  },
  {
    id: 'deploy',
    categoryKey: 'deploy',
    pages: [
      { id: 'text-to-image-api' },
      { id: 'batch-image-api' },
      { id: 'async-image-tasks' },
      { id: 'claude-code' },
      { id: 'codex' },
      { id: 'gemini-cli' },
      { id: 'sdk-quick' },
    ],
  },
  {
    id: 'tools',
    categoryKey: 'tools',
    pages: [
      { id: 'cc-switch' },
      { id: 'openclaw' },
      { id: 'hermes' },
      { id: 'cherry-studio' },
      { id: 'opencode' },
      { id: 'cline' },
      { id: 'continue' },
      { id: 'aider' },
    ],
  },
  {
    id: 'environment',
    categoryKey: 'environment',
    pages: [
      { id: 'nodejs-windows' },
      { id: 'nodejs-macos' },
      { id: 'nodejs-linux' },
      { id: 'network' },
    ],
  },
  {
    id: 'vibe-coding',
    categoryKey: 'vibe-coding',
    pages: [
      { id: 'start-here' },
      { id: 'learn-ai' },
      { id: 'instruct-ai' },
      { id: 'habits-ai' },
      { id: 'constrain-ai' },
      { id: 'context7' },
      { id: 'serena' },
      { id: 'playwright-mcp' },
      { id: 'exa' },
      { id: 'deepwiki' },
      { id: 'codegraph' },
      { id: 'codex-duo' },
      { id: 'official-plugins' },
      { id: 'understand-anything' },
      { id: 'skills-intro' },
      { id: 'superpowers' },
      { id: 'trellis' },
      { id: 'gsd' },
      { id: 'task-master' },
      { id: 'open-design' },
    ],
  },
]

export const PUBLIC_DOC_TREE_EN: PublicDocCategory[] = PUBLIC_DOC_TREE

export type PublicDocLocale = 'en' | 'zh'

const LEGACY_TUTORIAL_PAGE_CATEGORIES: Record<string, string> = {
  'text-to-image-api': 'deploy',
  'batch-image-api': 'deploy',
  'async-image-tasks': 'deploy',
}

export function normalizePublicDocLocation(catId: string, pageId: string) {
  if (catId === 'tutorial' && LEGACY_TUTORIAL_PAGE_CATEGORIES[pageId]) {
    return {
      catId: LEGACY_TUTORIAL_PAGE_CATEGORIES[pageId],
      pageId,
    }
  }
  return { catId, pageId }
}
