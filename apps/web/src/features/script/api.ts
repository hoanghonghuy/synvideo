import { ApiError } from '@/api/projects'

export type ScriptStatus = 'draft' | 'approved' | 'superseded'

export interface ScriptSection {
  key: string
  heading: string
  body: string
}

export interface ScriptEditableContent {
  sections: ScriptSection[]
  estimated_duration_seconds: number | null
  notes: string
}

export interface ScriptSummary {
  version: number
  revision: number
  status: ScriptStatus
  source_proposal_version: number
  content_locale: string
  created_at: string
  updated_at: string
  approved_at: string | null
}

export interface Script extends ScriptSummary, ScriptEditableContent {
  project_id: string
}

export interface PutScriptPayload extends ScriptEditableContent {
  revision: number
}

export interface TextGenerationOptionModel {
  id: string
  display_name: string
}

export interface TextGenerationOptionProvider {
  id: string
  display_name: string
  models: TextGenerationOptionModel[]
}

export interface TextGenerationOptionsResponse {
  providers: TextGenerationOptionProvider[]
}

export interface ScriptGenerationJob {
  id: string
  state: 'queued' | 'running' | 'succeeded' | 'failed'
  attempt: number
  max_attempts: number
  error_code: string | null
  script_version: number | null
  created_at: string
  updated_at: string
}

export interface CreateScriptGenerationPayload {
  request_id: string
  provider_id: string
  model_id: string
}

export function listScripts(projectId: string): Promise<ScriptSummary[]> {
  return request<ScriptSummary[]>(`/api/v1/projects/${projectId}/scripts`)
}

export function getScript(projectId: string, version: number): Promise<Script> {
  return request<Script>(`/api/v1/projects/${projectId}/scripts/${version}`)
}

export function putScript(projectId: string, version: number, payload: PutScriptPayload): Promise<Script> {
  return request<Script>(`/api/v1/projects/${projectId}/scripts/${version}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

export function approveScript(projectId: string, version: number, revision: number): Promise<Script> {
  return request<Script>(`/api/v1/projects/${projectId}/scripts/${version}/approve`, {
    method: 'POST',
    body: JSON.stringify({ revision }),
  })
}

export function getTextGenerationOptions(): Promise<TextGenerationOptionsResponse> {
  return request<TextGenerationOptionsResponse>('/api/v1/ai/text-generation-options')
}

export function createScriptGeneration(
  projectId: string,
  payload: CreateScriptGenerationPayload,
): Promise<ScriptGenerationJob> {
  return request<ScriptGenerationJob>(`/api/v1/projects/${projectId}/script-generations`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function getScriptGeneration(projectId: string, jobId: string): Promise<ScriptGenerationJob> {
  return request<ScriptGenerationJob>(`/api/v1/projects/${projectId}/script-generations/${jobId}`)
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
