import { beforeEach, describe, expect, it } from 'vitest'
import {
  STUDIO_DRAFT_TTL_MS,
  STUDIO_DRAFT_VERSION,
  clearStudioPromptDraft,
  clearStudioPendingJobId,
  clearStudioPendingJobForUser,
  getStudioDraftKey,
  getStudioPendingJobsForUser,
  getStudioPendingJobForUser,
  getStudioPendingJobSubmittedPrompt,
  loadStudioDraft,
  saveStudioDraft,
  setStudioPendingJobId,
  type ImageStudioDraft,
} from '@/utils/imageStudioSession'

const now = Date.parse('2026-07-16T12:00:00Z')
const draft: ImageStudioDraft = {
  userPrompt: 'matte black headphones',
  expertPrompt: 'product photography',
  expertOpen: true,
  templateId: 'commerce-white',
  accentColor: '#112233',
  aspect: '16:9',
  tier: '2K',
  count: 3,
}

describe('Image Studio session draft', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('stores only the versioned user-owned draft fields', () => {
    saveStudioDraft(42, draft, now)

    const persisted = JSON.parse(localStorage.getItem(getStudioDraftKey(42)) || '{}')
    expect(persisted).toEqual({
      version: STUDIO_DRAFT_VERSION,
      userId: 42,
      savedAt: now,
      draft,
    })
    expect(JSON.stringify(persisted)).not.toContain('apiKey')
    expect(JSON.stringify(persisted)).not.toContain('model')
    expect(JSON.stringify(persisted)).not.toContain('token')
    expect(JSON.stringify(persisted)).not.toContain('error')
  })

  it('restores a fresh draft only for its owner', () => {
    saveStudioDraft(42, draft, now)

    expect(loadStudioDraft(42, now + 1000)).toEqual(draft)
    expect(loadStudioDraft(99, now + 1000)).toBeNull()
  })

  it('safely ignores corrupt, expired, mismatched-owner, and future-version drafts', () => {
    const key = getStudioDraftKey(42)

    localStorage.setItem(key, '{broken')
    expect(loadStudioDraft(42, now)).toBeNull()

    localStorage.setItem(key, JSON.stringify({
      version: STUDIO_DRAFT_VERSION,
      userId: 42,
      savedAt: now - STUDIO_DRAFT_TTL_MS,
      draft,
    }))
    expect(loadStudioDraft(42, now)).toBeNull()

    localStorage.setItem(key, JSON.stringify({
      version: STUDIO_DRAFT_VERSION,
      userId: 7,
      savedAt: now,
      draft,
    }))
    expect(loadStudioDraft(42, now)).toBeNull()

    localStorage.setItem(key, JSON.stringify({
      version: STUDIO_DRAFT_VERSION + 1,
      userId: 42,
      savedAt: now,
      draft,
    }))
    expect(loadStudioDraft(42, now)).toBeNull()
  })

  it('clears prompt text after success while retaining reusable settings', () => {
    saveStudioDraft(42, draft, now)
    expect(clearStudioPromptDraft(42, {
      userPrompt: draft.userPrompt,
      expertPrompt: draft.expertPrompt,
    }, now + 1000)).toBe(true)

    expect(loadStudioDraft(42, now + 1000)).toEqual({
      ...draft,
      userPrompt: '',
      expertPrompt: '',
    })
  })

  it('does not clear a newer draft after an older task completes', () => {
    saveStudioDraft(42, { ...draft, userPrompt: 'new prompt' }, now)

    expect(clearStudioPromptDraft(42, {
      userPrompt: draft.userPrompt,
      expertPrompt: draft.expertPrompt,
    }, now + 1000)).toBe(false)
    expect(loadStudioDraft(42, now + 1000)?.userPrompt).toBe('new prompt')
  })

  it('does not clear matching text submitted with different generation settings', () => {
    saveStudioDraft(42, draft, now)

    expect(clearStudioPromptDraft(42, {
      userPrompt: draft.userPrompt,
      expertPrompt: draft.expertPrompt,
      templateId: 'free-create',
      aspect: draft.aspect,
      tier: draft.tier,
      count: draft.count,
    }, now + 1000)).toBe(false)
    expect(loadStudioDraft(42, now + 1000)?.userPrompt).toBe(draft.userPrompt)
  })

  it('scopes pending prompt snapshots to the matching job owner', () => {
    const submittedPrompt = {
      userPrompt: draft.userPrompt,
      expertPrompt: draft.expertPrompt,
    }
    setStudioPendingJobId('job-1', { userId: 42, submittedPrompt })

    expect(getStudioPendingJobSubmittedPrompt(42, 'job-1')).toEqual(submittedPrompt)
    expect(getStudioPendingJobForUser(42)).toEqual({ jobId: 'job-1', submittedPrompt })
    expect(getStudioPendingJobSubmittedPrompt(7, 'job-1')).toBeNull()
    expect(getStudioPendingJobSubmittedPrompt(42, 'job-2')).toBeNull()

    clearStudioPendingJobForUser(7, 'job-1')
    expect(getStudioPendingJobForUser(42)?.jobId).toBe('job-1')
    clearStudioPendingJobForUser(42, 'job-1')
    expect(getStudioPendingJobSubmittedPrompt(42, 'job-1')).toBeNull()
  })

  it('round-trips advanced generation settings in pending job snapshots', () => {
    const submittedPrompt = {
      userPrompt: draft.userPrompt,
      expertPrompt: draft.expertPrompt,
      background: 'transparent',
      outputFormat: 'webp',
      outputCompression: 82,
      inputFidelity: 'high',
      mode: 'edit' as const,
      referenceIds: ['ref-1', 'ref-2'],
    }

    setStudioPendingJobId('job-advanced', { userId: 42, submittedPrompt })

    expect(getStudioPendingJobSubmittedPrompt(42, 'job-advanced')).toEqual(submittedPrompt)
  })

  it('stores multiple pending jobs per user and clears only the terminal job', () => {
    setStudioPendingJobId('job-1', {
      userId: 42,
      submittedPrompt: { userPrompt: 'first prompt', expertPrompt: '' },
    })
    setStudioPendingJobId('job-2', {
      userId: 42,
      submittedPrompt: { userPrompt: 'second prompt', expertPrompt: 'second expert' },
    })
    setStudioPendingJobId('job-other-user', {
      userId: 7,
      submittedPrompt: { userPrompt: 'other account', expertPrompt: '' },
    })

    expect(getStudioPendingJobsForUser(42)).toEqual([
      {
        jobId: 'job-1',
        submittedPrompt: { userPrompt: 'first prompt', expertPrompt: '' },
      },
      {
        jobId: 'job-2',
        submittedPrompt: { userPrompt: 'second prompt', expertPrompt: 'second expert' },
      },
    ])
    expect(getStudioPendingJobsForUser(7).map((job) => job.jobId)).toEqual(['job-other-user'])

    clearStudioPendingJobForUser(42, 'job-1')
    expect(getStudioPendingJobsForUser(42).map((job) => job.jobId)).toEqual(['job-2'])
    expect(getStudioPendingJobsForUser(7).map((job) => job.jobId)).toEqual(['job-other-user'])
  })

  it('migrates the legacy single-job context only for its owning account', () => {
    setStudioPendingJobId('job-existing', {
      userId: 42,
      submittedPrompt: { userPrompt: 'existing prompt', expertPrompt: '' },
    })
    localStorage.setItem('image_studio_pending_job_id', 'job-legacy')
    localStorage.setItem('image_studio_pending_job_context:v1', JSON.stringify({
      version: 1,
      jobId: 'job-legacy',
      userId: 42,
      submittedPrompt: {
        userPrompt: 'legacy prompt',
        expertPrompt: 'legacy expert',
      },
    }))

    expect(getStudioPendingJobsForUser(7)).toEqual([])
    expect(localStorage.getItem('image_studio_pending_job_id')).toBe('job-legacy')

    expect(getStudioPendingJobsForUser(42)).toEqual([
      {
        jobId: 'job-existing',
        submittedPrompt: { userPrompt: 'existing prompt', expertPrompt: '' },
      },
      {
        jobId: 'job-legacy',
        submittedPrompt: { userPrompt: 'legacy prompt', expertPrompt: 'legacy expert' },
      },
    ])
    expect(localStorage.getItem('image_studio_pending_job_id')).toBeNull()
    expect(localStorage.getItem('image_studio_pending_job_context:v1')).toBeNull()
  })

  it('clears malformed pending context without exposing it to another user', () => {
    localStorage.setItem('image_studio_pending_job_id', 'job-1')
    localStorage.setItem('image_studio_pending_job_context:v1', '{broken')

    expect(getStudioPendingJobForUser(42)).toBeNull()
    clearStudioPendingJobId()
  })
})
