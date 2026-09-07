import type { RenderExportJob } from './api'

export const RENDER_EXPORT_POLL_MS = 1_500

export function renderExportStorageKey(projectID: string): string {
  return `synvideo:scene-editor:render-export:${projectID}`
}

export function isRenderExportTerminal(job: RenderExportJob | null): boolean {
  return job?.state === 'succeeded' || job?.state === 'failed'
}

export function persistRenderJobID(storage: Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>, projectID: string, jobID: string | null): void {
  const key = renderExportStorageKey(projectID)
  if (jobID) storage.setItem(key, jobID)
  else storage.removeItem(key)
}

export function restoreRenderJobID(storage: Pick<Storage, 'getItem'>, projectID: string): string | null {
  const value = storage.getItem(renderExportStorageKey(projectID))?.trim()
  return value || null
}
