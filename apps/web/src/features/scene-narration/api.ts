import type { MediaAsset } from '@/features/media/api'

export interface SceneNarrationBinding {
  id: string
  project_id: string
  scene_plan_version: number
  scene_key: string
  role: 'narration'
  binding_version: number
  asset_id: string
  status: 'active' | 'superseded'
  created_at: string
  superseded_at?: string
}

export interface SceneNarrationEntry {
  scene_key: string
  role: 'narration'
  binding?: SceneNarrationBinding
  asset?: MediaAsset
}

export interface TTSOptionVoice {
  id: string
  display_name: string
  locale?: string
  language?: string
  style?: string
}

export interface TTSOptionModel {
  id: string
  display_name: string
}

export interface TTSOptionProvider {
  id: string
  display_name: string
  models: TTSOptionModel[]
  voices: TTSOptionVoice[]
}

export interface TTSOptionsResponse {
  providers: TTSOptionProvider[]
}

export interface CreateSceneNarrationGenerationInput {
  request_id: string
  provider_id: string
  model_id: string
  voice_id: string
  format?: string
  assign_current: boolean
}

export interface SceneNarrationJobView {
  id: string
  state: 'queued' | 'running' | 'succeeded' | 'failed'
  attempt: number
  max_attempts: number
  error_code?: string
  media_asset_id?: string
  duration_seconds: number
  assigned_narration: boolean
  created_at: string
  updated_at: string
}

export interface ApiErrorPayload {
  error?: {
    code: string
    message: string
    fields?: Record<string, string>
  }
}

export class NarrationApiError extends Error {
  code: string
  status: number
  fields?: Record<string, string>

  constructor(status: number, code: string, message: string, fields?: Record<string, string>) {
    super(message)
    this.name = 'NarrationApiError'
    this.status = status
    this.code = code
    this.fields = fields
  }
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (res.status === 204) {
    return undefined as unknown as T
  }

  const data = (await res.json().catch(() => ({}))) as T & ApiErrorPayload

  if (!res.ok) {
    const error = (data as ApiErrorPayload).error
    throw new NarrationApiError(
      res.status,
      error?.code || 'UNKNOWN_ERROR',
      error?.message || `Request failed with status ${res.status}`,
      error?.fields,
    )
  }

  return data
}

export function audioContentURL(projectID: string, assetID: string): string {
  return `/api/v1/projects/${encodeURIComponent(projectID)}/media-assets/${encodeURIComponent(assetID)}/content`
}

export async function fetchTTSOptions(): Promise<TTSOptionsResponse> {
  const res = await fetch('/api/v1/ai/tts-options', {
    headers: {
      Accept: 'application/json',
    },
  })
  return handleResponse<TTSOptionsResponse>(res)
}

export async function listSceneNarrations(
  projectID: string,
  version: number,
): Promise<SceneNarrationEntry[]> {
  const res = await fetch(
    `/api/v1/projects/${encodeURIComponent(projectID)}/scene-plans/${encodeURIComponent(version)}/narration-bindings`,
    {
      headers: {
        Accept: 'application/json',
      },
    },
  )
  return handleResponse<SceneNarrationEntry[]>(res)
}

export async function createSceneNarrationGeneration(
  projectID: string,
  version: number,
  sceneKey: string,
  input: CreateSceneNarrationGenerationInput,
): Promise<SceneNarrationJobView> {
  const res = await fetch(
    `/api/v1/projects/${encodeURIComponent(projectID)}/scene-plans/${encodeURIComponent(version)}/scenes/${encodeURIComponent(sceneKey)}/narration-generations`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
  return handleResponse<SceneNarrationJobView>(res)
}

export async function getSceneNarrationGeneration(
  projectID: string,
  jobID: string,
): Promise<SceneNarrationJobView> {
  const res = await fetch(
    `/api/v1/projects/${encodeURIComponent(projectID)}/narration-generations/${encodeURIComponent(jobID)}`,
    {
      headers: {
        Accept: 'application/json',
      },
    },
  )
  return handleResponse<SceneNarrationJobView>(res)
}

export async function assignSceneNarration(
  projectID: string,
  version: number,
  sceneKey: string,
  assetID: string,
): Promise<SceneNarrationEntry> {
  const res = await fetch(
    `/api/v1/projects/${encodeURIComponent(projectID)}/scene-plans/${encodeURIComponent(version)}/scenes/${encodeURIComponent(sceneKey)}/narration`,
    {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify({ asset_id: assetID }),
    },
  )
  return handleResponse<SceneNarrationEntry>(res)
}

export async function listSceneNarrationHistory(
  projectID: string,
  version: number,
  sceneKey: string,
): Promise<SceneNarrationEntry[]> {
  const res = await fetch(
    `/api/v1/projects/${encodeURIComponent(projectID)}/scene-plans/${encodeURIComponent(version)}/scenes/${encodeURIComponent(sceneKey)}/narration/history`,
    {
      headers: {
        Accept: 'application/json',
      },
    },
  )
  return handleResponse<SceneNarrationEntry[]>(res)
}
