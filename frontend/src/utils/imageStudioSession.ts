export const STUDIO_PENDING_JOB_KEY = 'image_studio_pending_job_id'

export function getStudioPendingJobId(): string | null {
  return localStorage.getItem(STUDIO_PENDING_JOB_KEY)
}

export function setStudioPendingJobId(jobId: string) {
  localStorage.setItem(STUDIO_PENDING_JOB_KEY, jobId)
}

export function clearStudioPendingJobId() {
  localStorage.removeItem(STUDIO_PENDING_JOB_KEY)
}
