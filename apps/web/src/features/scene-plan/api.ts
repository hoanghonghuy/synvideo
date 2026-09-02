import { ApiError } from '@/api/projects'

export type ScenePlanStatus = 'draft' | 'approved' | 'superseded'

export type PlannedSourceType = 'stock' | 'upload' | 'creator_media' | 'generated_image' | 'generated_video'

export interface Scene {
  key: string
  script_section_key: string
  narration: string
  visual_instruction: string
  planned_source_type: PlannedSourceType
  expected_duration_seconds: number
  caption_intent?: string
  transition_notes?: string
}

export interface ScenePlanSummary {
  version: number
  revision: number
  status: ScenePlanStatus
  source_script_version: number
  source_proposal_version: number
  content_locale: string
  created_at: string
  updated_at: string
  approved_at: string | null
}

export interface ScenePlan extends ScenePlanSummary {
  project_id: string
  scenes: Scene[]
}

export interface PutScenePlanPayload {
  revision: number
  scenes: Scene[]
}

export interface ScenePlanGenerationJob {
  id: string
  state: 'queued' | 'running' | 'succeeded' | 'failed'
  attempt: number
  max_attempts: number
  error_code: string | null
  scene_plan_version: number | null
  created_at: string
  updated_at: string
}

export interface CreateScenePlanGenerationPayload {
  request_id: string
  provider_id: string
  model_id: string
}

export function listScenePlans(projectId: string): Promise<ScenePlanSummary[]> {
  return request<ScenePlanSummary[]>(`/api/v1/projects/${projectId}/scene-plans`)
}

export function getScenePlan(projectId: string, version: number): Promise<ScenePlan> {
  return request<ScenePlan>(`/api/v1/projects/${projectId}/scene-plans/${version}`)
}

export function putScenePlan(
  projectId: string,
  version: number,
  payload: PutScenePlanPayload,
): Promise<ScenePlan> {
  return request<ScenePlan>(`/api/v1/projects/${projectId}/scene-plans/${version}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

export function approveScenePlan(
  projectId: string,
  version: number,
  revision: number,
): Promise<ScenePlan> {
  return request<ScenePlan>(`/api/v1/projects/${projectId}/scene-plans/${version}/approve`, {
    method: 'POST',
    body: JSON.stringify({ revision }),
  })
}

export function createScenePlanGeneration(
  projectId: string,
  payload: CreateScenePlanGenerationPayload,
): Promise<ScenePlanGenerationJob> {
  return request<ScenePlanGenerationJob>(`/api/v1/projects/${projectId}/scene-plan-generations`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function getScenePlanGeneration(
  projectId: string,
  jobId: string,
): Promise<ScenePlanGenerationJob> {
  return request<ScenePlanGenerationJob>(`/api/v1/projects/${projectId}/scene-plan-generations/${jobId}`)
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
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
