import type { RenderExportJob } from './api'

export const RENDER_EXPORT_POLL_MS = 1_500

type RenderStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export function renderExportStorageKey(projectID: string): string {
  return `synvideo:scene-editor:render-export:${projectID}`
}

export function renderRetryRequestStorageKey(projectID: string, sourceJobID: string): string {
  return `synvideo:scene-editor:render-retry:${projectID}:${sourceJobID}`
}

export function isRenderExportTerminal(job: RenderExportJob | null): boolean {
  return job?.state === 'succeeded' || job?.state === 'failed' || job?.state === 'cancelled'
}

export function isRenderExportCancellable(job: RenderExportJob | null): boolean {
  return job?.state === 'queued' || (job?.state === 'running' && !job.cancellation_pending)
}

export function isRenderExportRetryable(job: RenderExportJob | null): boolean {
  return isRenderExportTerminal(job)
}

export function persistRenderJobID(storage: RenderStorage, projectID: string, jobID: string | null): void {
  const key = renderExportStorageKey(projectID)
  if (jobID) storage.setItem(key, jobID)
  else storage.removeItem(key)
}

export function restoreRenderJobID(storage: Pick<Storage, 'getItem'>, projectID: string): string | null {
  const value = storage.getItem(renderExportStorageKey(projectID))?.trim()
  return value || null
}

export function resolveRenderRetryRequestID(
  storage: RenderStorage | null,
  projectID: string,
  sourceJobID: string,
  proposedRequestID: string,
): string {
  if (!storage) return proposedRequestID
  const key = renderRetryRequestStorageKey(projectID, sourceJobID)
  const existing = storage.getItem(key)?.trim()
  if (existing) return existing
  storage.setItem(key, proposedRequestID)
  return proposedRequestID
}

export function clearRenderRetryRequestID(storage: RenderStorage | null, projectID: string, sourceJobID: string): void {
  storage?.removeItem(renderRetryRequestStorageKey(projectID, sourceJobID))
}
