import { beforeEach, describe, expect, it } from 'vitest'
import {
  STUDIO_DRAFT_TTL_MS,
  STUDIO_DRAFT_VERSION,
  clearStudioPromptDraft,
  clearStudioPendingJobId,
  clearStudioPendingJobForUser,
  getStudioDraftKey,
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

  it('clears malformed pending context without exposing it to another user', () => {
    localStorage.setItem('image_studio_pending_job_id', 'job-1')
    localStorage.setItem('image_studio_pending_job_context:v1', '{broken')

    expect(getStudioPendingJobForUser(42)).toBeNull()
    clearStudioPendingJobId()
  })
})
