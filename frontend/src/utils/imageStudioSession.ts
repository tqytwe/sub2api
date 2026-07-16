export const STUDIO_PENDING_JOB_KEY = 'image_studio_pending_job_id'
export const STUDIO_DRAFT_VERSION = 1
export const STUDIO_DRAFT_TTL_MS = 7 * 24 * 60 * 60 * 1000

const STUDIO_DRAFT_KEY_PREFIX = `image_studio_draft:v${STUDIO_DRAFT_VERSION}:user:`
const STUDIO_PENDING_JOB_CONTEXT_KEY = 'image_studio_pending_job_context:v1'

export interface ImageStudioDraft {
  userPrompt: string
  expertPrompt: string
  expertOpen: boolean
  templateId: string | null
  accentColor: string
  aspect: string
  tier: string
  count: number
}

interface ImageStudioDraftEnvelope {
  version: number
  userId: number
  savedAt: number
  draft: ImageStudioDraft
}

export interface ImageStudioSubmittedPrompt {
  userPrompt: string
  expertPrompt: string
}

interface ImageStudioPendingJobContext {
  version: 1
  jobId: string
  userId: number
  submittedPrompt: ImageStudioSubmittedPrompt
}

export interface ImageStudioPendingJob {
  jobId: string
  submittedPrompt: ImageStudioSubmittedPrompt
}

export function getStudioDraftKey(userId: number): string {
  return `${STUDIO_DRAFT_KEY_PREFIX}${userId}`
}

function isImageStudioDraft(value: unknown): value is ImageStudioDraft {
  if (!value || typeof value !== 'object') return false
  const draft = value as Partial<ImageStudioDraft>
  return (
    typeof draft.userPrompt === 'string'
    && typeof draft.expertPrompt === 'string'
    && typeof draft.expertOpen === 'boolean'
    && (draft.templateId === null || typeof draft.templateId === 'string')
    && typeof draft.accentColor === 'string'
    && typeof draft.aspect === 'string'
    && typeof draft.tier === 'string'
    && Number.isInteger(draft.count)
    && Number(draft.count) > 0
  )
}

export function loadStudioDraft(userId: number, now = Date.now()): ImageStudioDraft | null {
  const key = getStudioDraftKey(userId)
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    const envelope = JSON.parse(raw) as Partial<ImageStudioDraftEnvelope>
    const valid = (
      envelope.version === STUDIO_DRAFT_VERSION
      && envelope.userId === userId
      && typeof envelope.savedAt === 'number'
      && Number.isFinite(envelope.savedAt)
      && envelope.savedAt <= now
      && now - envelope.savedAt < STUDIO_DRAFT_TTL_MS
      && isImageStudioDraft(envelope.draft)
    )
    if (!valid) {
      localStorage.removeItem(key)
      return null
    }
    return envelope.draft as ImageStudioDraft
  } catch {
    try {
      localStorage.removeItem(key)
    } catch {
      // Storage can be unavailable in hardened browser contexts.
    }
    return null
  }
}

export function saveStudioDraft(userId: number, draft: ImageStudioDraft, now = Date.now()): void {
  if (!isImageStudioDraft(draft)) return
  const envelope: ImageStudioDraftEnvelope = {
    version: STUDIO_DRAFT_VERSION,
    userId,
    savedAt: now,
    draft,
  }
  try {
    localStorage.setItem(getStudioDraftKey(userId), JSON.stringify(envelope))
  } catch {
    // Draft persistence must never block image generation.
  }
}

export function clearStudioPromptDraft(
  userId: number,
  expected: ImageStudioSubmittedPrompt,
  now = Date.now(),
): boolean {
  const draft = loadStudioDraft(userId, now)
  if (
    !draft
    || draft.userPrompt !== expected.userPrompt
    || draft.expertPrompt !== expected.expertPrompt
  ) {
    return false
  }
  saveStudioDraft(userId, {
    ...draft,
    userPrompt: '',
    expertPrompt: '',
  }, now)
  return true
}

export function getStudioPendingJobId(): string | null {
  return localStorage.getItem(STUDIO_PENDING_JOB_KEY)
}

export function setStudioPendingJobId(
  jobId: string,
  context?: { userId: number; submittedPrompt: ImageStudioSubmittedPrompt },
) {
  localStorage.setItem(STUDIO_PENDING_JOB_KEY, jobId)
  if (!context) {
    localStorage.removeItem(STUDIO_PENDING_JOB_CONTEXT_KEY)
    return
  }
  const value: ImageStudioPendingJobContext = {
    version: 1,
    jobId,
    userId: context.userId,
    submittedPrompt: context.submittedPrompt,
  }
  localStorage.setItem(STUDIO_PENDING_JOB_CONTEXT_KEY, JSON.stringify(value))
}

export function getStudioPendingJobSubmittedPrompt(
  userId: number,
  jobId: string,
): ImageStudioSubmittedPrompt | null {
  const pending = getStudioPendingJobForUser(userId)
  return pending?.jobId === jobId ? pending.submittedPrompt : null
}

export function getStudioPendingJobForUser(userId: number): ImageStudioPendingJob | null {
  try {
    const raw = localStorage.getItem(STUDIO_PENDING_JOB_CONTEXT_KEY)
    if (!raw) return null
    const value = JSON.parse(raw) as Partial<ImageStudioPendingJobContext>
    if (
      value.version !== 1
      || value.userId !== userId
      || typeof value.jobId !== 'string'
      || localStorage.getItem(STUDIO_PENDING_JOB_KEY) !== value.jobId
      || !value.submittedPrompt
      || typeof value.submittedPrompt.userPrompt !== 'string'
      || typeof value.submittedPrompt.expertPrompt !== 'string'
    ) {
      return null
    }
    return {
      jobId: value.jobId,
      submittedPrompt: value.submittedPrompt,
    }
  } catch {
    return null
  }
}

export function clearStudioPendingJobId() {
  localStorage.removeItem(STUDIO_PENDING_JOB_KEY)
  localStorage.removeItem(STUDIO_PENDING_JOB_CONTEXT_KEY)
}

export function clearStudioPendingJobForUser(userId: number, jobId: string): void {
  const pending = getStudioPendingJobForUser(userId)
  if (!pending || pending.jobId !== jobId) return
  clearStudioPendingJobId()
}
