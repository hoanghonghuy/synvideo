import { apiFetch } from '@/api/http'
import { ApiError } from '@/api/projects'

import { clearRenderRetryRequestID, resolveRenderRetryRequestID } from './renderExportState'

export type SceneEditorState = 'CURRENT' | 'STALE' | 'BROKEN'
export type SceneEditorFit = 'contain' | 'cover'
export type SceneEditorTransitionKind = 'cut' | 'fade' | 'crossfade'
export type RenderExportState = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
export type RenderSubtitleMode = 'off' | 'webvtt'

export interface SceneEditorVisualRef {
  asset_id: string
  binding_id: string
}

export interface SceneEditorNarrationRef {
  asset_id: string
  binding_id: string
  lineage_id: string
  duration_ms: number
}

export interface SceneEditorCaptionRef {
  document_id: string
  revision: number
  lineage_id: string
  last_end_ms: number
}

export interface SceneEditorAudioMixRef {
  document_id: string
  revision: number
  music_asset_id: string
  narration_lineage_id: string
}

export interface SceneEditorCrop {
  x: number
  y: number
  width: number
  height: number
}

export interface SceneEditorVisualTreatment {
  fit: SceneEditorFit
  crop?: SceneEditorCrop
  position_x: number
  position_y: number
  scale: number
  mute_video: boolean
}

export interface SceneEditorTransition {
  kind: SceneEditorTransitionKind
  duration_ms: number
}

export interface SceneEditorScene {
  id: string
  scene_key: string
  visual?: SceneEditorVisualRef
  narration?: SceneEditorNarrationRef
  caption?: SceneEditorCaptionRef
  duration_ms: number
  visual_treatment: SceneEditorVisualTreatment
  transition_out: SceneEditorTransition
  notes?: string
}

export interface SceneEditorDocument {
  id: string
  project_id: string
  revision: number
  scene_plan_version: number
  scenes: SceneEditorScene[]
  audio_mix?: SceneEditorAudioMixRef
  created_at: string
  updated_at: string
}

export interface SceneEditorView extends SceneEditorDocument {
  state: SceneEditorState
}

export interface SceneEditorSnapshot {
  schema_version: number
  composition_id: string
  revision: number
  project_id: string
  scene_plan_version: number
  scenes: SceneEditorScene[]
  audio_mix?: SceneEditorAudioMixRef
  digest: string
}

export interface SceneEditorCandidate {
  scene_plan_version: number
  scenes: Array<{
    scene_key: string
    visual?: SceneEditorVisualRef
    narration?: SceneEditorNarrationRef
    caption?: SceneEditorCaptionRef
  }>
  audio_mix?: SceneEditorAudioMixRef
}

export interface SceneEditorReconcilePreview {
  from_revision: number
  from_scene_plan_version: number
  to_scene_plan_version: number
  changes: Array<{
    composition_scene_id: string
    scene_key: string
    reasons: string[]
    preserves_edits: boolean
  }>
  audio_mix_changed: boolean
  ambiguous: boolean
  preview_digest: string
}

export interface RenderExportArtifact {
  id: string
  media_asset_id: string
  subtitle_media_asset_id?: string
  byte_size: number
  sha256: string
  mime_type: string
  duration_ms: number
  width: number
  height: number
  toolchain_version: string
  created_at: string
}

export interface RenderExportJob {
  id: string
  state: RenderExportState
  attempt: number
  max_attempts: number
  error_code?: string
  snapshot_digest: string
  profile_id: string
  subtitle_mode: RenderSubtitleMode
  retry_of_render_job_id?: string
  cancellation_pending?: boolean
  artifact?: RenderExportArtifact
  created_at: string
  updated_at: string
}

export interface RenderExportHistory {
  items: RenderExportJob[]
  next_cursor?: string
}

const base = (projectID: string) => `/api/v1/projects/${encodeURIComponent(projectID)}/scene-editor`
const renderBase = (projectID: string) => `/api/v1/projects/${encodeURIComponent(projectID)}/render-exports`

export async function getSceneEditor(projectID: string): Promise<SceneEditorView> {
  return request<SceneEditorView>(base(projectID))
}

export async function updateSceneEditor(
  projectID: string,
  input: { expected_revision: number; scenes: SceneEditorScene[]; audio_mix: SceneEditorAudioMixRef | undefined },
): Promise<SceneEditorView> {
  return request<SceneEditorView>(base(projectID), { method: 'PUT', body: JSON.stringify(input) })
}

export async function reorderScene(projectID: string, sceneID: string, expectedRevision: number, to: number): Promise<SceneEditorView> {
  return request<SceneEditorView>(`${base(projectID)}/scenes/${encodeURIComponent(sceneID)}/reorder`, {
    method: 'POST',
    body: JSON.stringify({ expected_revision: expectedRevision, to }),
  })
}

export async function duplicateScene(projectID: string, sceneID: string, expectedRevision: number): Promise<SceneEditorView> {
  return request<SceneEditorView>(`${base(projectID)}/scenes/${encodeURIComponent(sceneID)}/duplicate`, {
    method: 'POST',
    body: JSON.stringify({ expected_revision: expectedRevision }),
  })
}

export async function removeScene(projectID: string, sceneID: string, expectedRevision: number): Promise<SceneEditorView> {
  return request<SceneEditorView>(`${base(projectID)}/scenes/${encodeURIComponent(sceneID)}/remove`, {
    method: 'POST',
    body: JSON.stringify({ expected_revision: expectedRevision }),
  })
}

export async function previewSceneEditorReconcile(projectID: string, candidate: SceneEditorCandidate): Promise<SceneEditorReconcilePreview> {
  return request<SceneEditorReconcilePreview>(`${base(projectID)}/reconcile/preview`, {
    method: 'POST',
    body: JSON.stringify({ candidate }),
  })
}

export async function reconcileSceneEditor(
  projectID: string,
  expectedRevision: number,
  candidate: SceneEditorCandidate,
  previewDigest: string,
): Promise<SceneEditorView> {
  return request<SceneEditorView>(`${base(projectID)}/reconcile`, {
    method: 'POST',
    body: JSON.stringify({ expected_revision: expectedRevision, preview_digest: previewDigest, candidate }),
  })
}

export async function createSceneEditorSnapshot(projectID: string, expectedRevision: number): Promise<SceneEditorSnapshot> {
  return request<SceneEditorSnapshot>(`${base(projectID)}/snapshots`, {
    method: 'POST',
    body: JSON.stringify({ expected_revision: expectedRevision }),
  })
}

export async function createRenderExport(projectID: string, snapshotDigest: string, subtitleMode: RenderSubtitleMode = 'off'): Promise<RenderExportJob> {
  return request<RenderExportJob>(renderBase(projectID), {
    method: 'POST',
    body: JSON.stringify({ snapshot_digest: snapshotDigest, subtitle_mode: subtitleMode }),
  })
}

export async function getRenderExport(projectID: string, jobID: string): Promise<RenderExportJob> {
  return request<RenderExportJob>(`${renderBase(projectID)}/${encodeURIComponent(jobID)}`)
}

export async function listRenderExportHistory(projectID: string, limit = 20, cursor?: string): Promise<RenderExportHistory> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (cursor) params.set('cursor', cursor)
  const query = params.toString()
  return request<RenderExportHistory>(`${renderBase(projectID)}?${query}`)
}

export async function cancelRenderExport(projectID: string, jobID: string): Promise<RenderExportJob> {
  return request<RenderExportJob>(`${renderBase(projectID)}/${encodeURIComponent(jobID)}/cancel`, { method: 'POST' })
}

export async function retryRenderExport(projectID: string, sourceJobID: string, requestID: string): Promise<RenderExportJob> {
  const storage = typeof window === 'undefined' ? null : window.sessionStorage
  const logicalRequestID = resolveRenderRetryRequestID(storage, projectID, sourceJobID, requestID)

  try {
    const retried = await request<RenderExportJob>(`${renderBase(projectID)}/${encodeURIComponent(sourceJobID)}/retry`, {
      method: 'POST',
      body: JSON.stringify({ request_id: logicalRequestID }),
    })
    clearRenderRetryRequestID(storage, projectID, sourceJobID)
    return retried
  } catch (cause) {
    if (cause instanceof ApiError && cause.status >= 400 && cause.status < 500) {
      clearRenderRetryRequestID(storage, projectID, sourceJobID)
    }
    throw cause
  }
}

export function mediaAssetContentURL(projectID: string, assetID: string): string {
  return `/api/v1/projects/${encodeURIComponent(projectID)}/media-assets/${encodeURIComponent(assetID)}/content`
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const response = await apiFetch(url, {
    ...init,
    headers: { 'Content-Type': 'application/json', Accept: 'application/json', ...init.headers },
  })
  const body = await response.json().catch(() => null)
  if (!response.ok) {
    const error = isErrorEnvelope(body) ? body.error : undefined
    throw new ApiError(response.status, error?.code ?? 'request_failed', error?.message ?? 'Request failed.', error?.fields ?? {})
  }
  return body as T
}

function isErrorEnvelope(value: unknown): value is { error?: { code?: string; message?: string; fields?: Record<string, string> } } {
  return typeof value === 'object' && value !== null && 'error' in value
}
