export const STUDIO_PENDING_JOB_KEY = 'image_studio_pending_job_id'
export const STUDIO_DRAFT_VERSION = 1
export const STUDIO_DRAFT_TTL_MS = 7 * 24 * 60 * 60 * 1000
export const STUDIO_PENDING_JOBS_VERSION = 2

const STUDIO_DRAFT_KEY_PREFIX = `image_studio_draft:v${STUDIO_DRAFT_VERSION}:user:`
const STUDIO_PENDING_JOB_CONTEXT_KEY = 'image_studio_pending_job_context:v1'
const STUDIO_PENDING_JOBS_KEY_PREFIX = `image_studio_pending_jobs:v${STUDIO_PENDING_JOBS_VERSION}:user:`

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
  expertOpen?: boolean
  templateId?: string | null
  accentColor?: string
  aspect?: string
  tier?: string
  count?: number
  model?: string
  quality?: string
  apiKeyId?: number
  background?: string
  outputFormat?: string
  outputCompression?: number | null
  inputFidelity?: string
  mode?: 'create' | 'edit'
  referenceIds?: string[]
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

interface ImageStudioPendingJobsEnvelope {
  version: typeof STUDIO_PENDING_JOBS_VERSION
  userId: number
  jobs: ImageStudioPendingJob[]
}

export function getStudioDraftKey(userId: number): string {
  return `${STUDIO_DRAFT_KEY_PREFIX}${userId}`
}

export function getStudioPendingJobsKey(userId: number): string {
  return `${STUDIO_PENDING_JOBS_KEY_PREFIX}${userId}`
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
    || (expected.expertOpen !== undefined && draft.expertOpen !== expected.expertOpen)
    || (expected.templateId !== undefined && draft.templateId !== expected.templateId)
    || (expected.accentColor !== undefined && draft.accentColor !== expected.accentColor)
    || (expected.aspect !== undefined && draft.aspect !== expected.aspect)
    || (expected.tier !== undefined && draft.tier !== expected.tier)
    || (expected.count !== undefined && draft.count !== expected.count)
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

function isSubmittedPrompt(value: unknown): value is ImageStudioSubmittedPrompt {
  if (!value || typeof value !== 'object') return false
  const prompt = value as Partial<ImageStudioSubmittedPrompt>
  return (
    typeof prompt.userPrompt === 'string'
    && typeof prompt.expertPrompt === 'string'
    && (prompt.expertOpen === undefined || typeof prompt.expertOpen === 'boolean')
    && (prompt.templateId === undefined || prompt.templateId === null || typeof prompt.templateId === 'string')
    && (prompt.accentColor === undefined || typeof prompt.accentColor === 'string')
    && (prompt.aspect === undefined || typeof prompt.aspect === 'string')
    && (prompt.tier === undefined || typeof prompt.tier === 'string')
    && (prompt.count === undefined || (Number.isInteger(prompt.count) && Number(prompt.count) > 0))
    && (prompt.model === undefined || typeof prompt.model === 'string')
    && (prompt.quality === undefined || typeof prompt.quality === 'string')
    && (prompt.apiKeyId === undefined || (Number.isInteger(prompt.apiKeyId) && Number(prompt.apiKeyId) > 0))
    && (prompt.background === undefined || typeof prompt.background === 'string')
    && (prompt.outputFormat === undefined || typeof prompt.outputFormat === 'string')
    && (
      prompt.outputCompression === undefined
      || prompt.outputCompression === null
      || (typeof prompt.outputCompression === 'number' && Number.isFinite(prompt.outputCompression))
    )
    && (prompt.inputFidelity === undefined || typeof prompt.inputFidelity === 'string')
    && (
      prompt.mode === undefined
      || prompt.mode === 'create'
      || prompt.mode === 'edit'
    )
    && (
      prompt.referenceIds === undefined
      || (
        Array.isArray(prompt.referenceIds)
        && prompt.referenceIds.every((id) => typeof id === 'string' && id.length > 0)
      )
    )
  )
}

function normalizePendingJobs(value: unknown): ImageStudioPendingJob[] {
  if (!Array.isArray(value)) return []
  const jobs: ImageStudioPendingJob[] = []
  const seen = new Set<string>()
  for (const entry of value) {
    if (!entry || typeof entry !== 'object') continue
    const pending = entry as Partial<ImageStudioPendingJob>
    if (
      typeof pending.jobId !== 'string'
      || !pending.jobId
      || seen.has(pending.jobId)
      || !isSubmittedPrompt(pending.submittedPrompt)
    ) {
      continue
    }
    seen.add(pending.jobId)
    jobs.push({
      jobId: pending.jobId,
      submittedPrompt: pending.submittedPrompt,
    })
  }
  return jobs
}

function readPendingJobs(userId: number): ImageStudioPendingJob[] {
  const key = getStudioPendingJobsKey(userId)
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return []
    const envelope = JSON.parse(raw) as Partial<ImageStudioPendingJobsEnvelope>
    if (envelope.version !== STUDIO_PENDING_JOBS_VERSION || envelope.userId !== userId) {
      localStorage.removeItem(key)
      return []
    }
    return normalizePendingJobs(envelope.jobs)
  } catch {
    try {
      localStorage.removeItem(key)
    } catch {
      // Storage can be unavailable in hardened browser contexts.
    }
    return []
  }
}

function writePendingJobs(userId: number, jobs: ImageStudioPendingJob[]): void {
  try {
    const key = getStudioPendingJobsKey(userId)
    const normalized = normalizePendingJobs(jobs)
    if (!normalized.length) {
      localStorage.removeItem(key)
      return
    }
    const envelope: ImageStudioPendingJobsEnvelope = {
      version: STUDIO_PENDING_JOBS_VERSION,
      userId,
      jobs: normalized,
    }
    localStorage.setItem(key, JSON.stringify(envelope))
  } catch {
    // Pending recovery is best effort and must not block generation.
  }
}

function migrateLegacyPendingJob(userId: number, jobs: ImageStudioPendingJob[]): ImageStudioPendingJob[] {
  try {
    const raw = localStorage.getItem(STUDIO_PENDING_JOB_CONTEXT_KEY)
    if (!raw) return jobs
    const value = JSON.parse(raw) as Partial<ImageStudioPendingJobContext>
    const legacyJobId = localStorage.getItem(STUDIO_PENDING_JOB_KEY)
    if (
      value.version !== 1
      || value.userId !== userId
      || typeof value.jobId !== 'string'
      || value.jobId !== legacyJobId
      || !isSubmittedPrompt(value.submittedPrompt)
    ) {
      return jobs
    }
    const migrated = jobs.some((job) => job.jobId === value.jobId)
      ? jobs
      : [...jobs, {
        jobId: value.jobId,
        submittedPrompt: value.submittedPrompt,
      }]
    writePendingJobs(userId, migrated)
    localStorage.removeItem(STUDIO_PENDING_JOB_KEY)
    localStorage.removeItem(STUDIO_PENDING_JOB_CONTEXT_KEY)
    return migrated
  } catch {
    return jobs
  }
}

export function getStudioPendingJobsForUser(userId: number): ImageStudioPendingJob[] {
  return migrateLegacyPendingJob(userId, readPendingJobs(userId))
}

export function setStudioPendingJobId(
  jobId: string,
  context?: { userId: number; submittedPrompt: ImageStudioSubmittedPrompt },
) {
  if (!context) {
    localStorage.setItem(STUDIO_PENDING_JOB_KEY, jobId)
    localStorage.removeItem(STUDIO_PENDING_JOB_CONTEXT_KEY)
    return
  }
  const jobs = getStudioPendingJobsForUser(context.userId)
  const pending: ImageStudioPendingJob = {
    jobId,
    submittedPrompt: context.submittedPrompt,
  }
  const index = jobs.findIndex((job) => job.jobId === jobId)
  if (index >= 0) jobs[index] = pending
  else jobs.push(pending)
  writePendingJobs(context.userId, jobs)
}

export function getStudioPendingJobSubmittedPrompt(
  userId: number,
  jobId: string,
): ImageStudioSubmittedPrompt | null {
  return getStudioPendingJobsForUser(userId)
    .find((pending) => pending.jobId === jobId)
    ?.submittedPrompt ?? null
}

export function getStudioPendingJobForUser(userId: number): ImageStudioPendingJob | null {
  return getStudioPendingJobsForUser(userId)[0] ?? null
}

export function clearStudioPendingJobId() {
  localStorage.removeItem(STUDIO_PENDING_JOB_KEY)
  localStorage.removeItem(STUDIO_PENDING_JOB_CONTEXT_KEY)
}

export function clearStudioPendingJobForUser(userId: number, jobId: string): void {
  const jobs = getStudioPendingJobsForUser(userId)
  const next = jobs.filter((pending) => pending.jobId !== jobId)
  if (next.length !== jobs.length) writePendingJobs(userId, next)
  try {
    const raw = localStorage.getItem(STUDIO_PENDING_JOB_CONTEXT_KEY)
    if (!raw) return
    const legacy = JSON.parse(raw) as Partial<ImageStudioPendingJobContext>
    if (
      legacy.userId === userId
      && legacy.jobId === jobId
      && localStorage.getItem(STUDIO_PENDING_JOB_KEY) === jobId
    ) {
      clearStudioPendingJobId()
    }
  } catch {
    // Leave malformed legacy data isolated from the versioned user bucket.
  }
}
