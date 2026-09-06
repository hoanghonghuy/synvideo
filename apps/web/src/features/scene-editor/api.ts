import { ApiError } from '@/api/projects'

export type SceneEditorState = 'CURRENT' | 'STALE' | 'BROKEN'
export type SceneEditorFit = 'contain' | 'cover'
export type SceneEditorTransitionKind = 'cut' | 'fade' | 'crossfade'

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
}

const base = (projectID: string) => `/api/v1/projects/${encodeURIComponent(projectID)}/scene-editor`

export async function getSceneEditor(projectID: string): Promise<SceneEditorView> {
  return request<SceneEditorView>(base(projectID))
}

export async function updateSceneEditor(projectID: string, input: { expected_revision: number; scenes: SceneEditorScene[]; audio_mix?: SceneEditorAudioMixRef }): Promise<SceneEditorView> {
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

export async function reconcileSceneEditor(projectID: string, expectedRevision: number, candidate: SceneEditorCandidate): Promise<SceneEditorView> {
  return request<SceneEditorView>(`${base(projectID)}/reconcile`, {
    method: 'POST',
    body: JSON.stringify({ expected_revision: expectedRevision, candidate }),
  })
}

export async function createSceneEditorSnapshot(projectID: string, expectedRevision: number): Promise<SceneEditorSnapshot> {
  return request<SceneEditorSnapshot>(`${base(projectID)}/snapshots`, {
    method: 'POST',
    body: JSON.stringify({ expected_revision: expectedRevision }),
  })
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(url, {
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
