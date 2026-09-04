import { ApiError } from '@/api/projects'

export interface VideoGenerationOptionModel {
  id: string
  display_name: string
  min_duration_seconds?: number
  max_duration_seconds?: number
}

export interface VideoGenerationOptionProvider {
  id: string
  display_name: string
  models: VideoGenerationOptionModel[]
}

export interface VideoGenerationOptionsResponse {
  providers: VideoGenerationOptionProvider[]
}

export interface CreateSceneVideoGenerationInput {
  request_id: string
  provider_id: string
  model_id: string
  duration_seconds?: number
  assign_primary_visual: boolean
}

export interface SceneVideoJobView {
  id: string
  state: 'queued' | 'running' | 'succeeded' | 'failed'
  attempt: number
  max_attempts: number
  error_code?: string | null
  media_asset_id?: string
  external_operation_id?: string
  assigned_primary_visual: boolean
  created_at: string
  updated_at: string
}

export function fetchVideoGenerationOptions(): Promise<VideoGenerationOptionsResponse> {
  return request<VideoGenerationOptionsResponse>('/api/v1/ai/video-generation-options')
}

export function createSceneVideoGeneration(
  projectId: string,
  version: number,
  sceneKey: string,
  input: CreateSceneVideoGenerationInput,
): Promise<SceneVideoJobView> {
  return request<SceneVideoJobView>(
    `/api/v1/projects/${encodeURIComponent(projectId)}/scene-plans/${version}/scenes/${encodeURIComponent(sceneKey)}/video-generations`,
    {
      method: 'POST',
      body: JSON.stringify(input),
    },
  )
}

export function getSceneVideoGeneration(projectId: string, jobId: string): Promise<SceneVideoJobView> {
  return request<SceneVideoJobView>(
    `/api/v1/projects/${encodeURIComponent(projectId)}/video-generations/${encodeURIComponent(jobId)}`,
  )
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      ...init.headers,
    },
  })
  const body = await response.json().catch(() => null)
  if (!response.ok) {
    const error = body?.error
    throw new ApiError(
      response.status,
      error?.code ?? 'request_failed',
      error?.message ?? 'Request failed.',
      error?.fields ?? {},
    )
  }
  return body as T
}
