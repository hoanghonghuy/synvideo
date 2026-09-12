import { apiFetch } from '@/api/http'
import { ApiError } from '@/api/projects'

export type ConnectionState = 'connected' | 'reconnect_required' | 'revoked'
export type PublishState =
  | 'queued'
  | 'uploading'
  | 'upload_accepted'
  | 'processing'
  | 'private'
  | 'scheduled'
  | 'public'
  | 'reconnect_required'
  | 'retryable_failure'
  | 'rejected'

export interface PublishingCapabilities {
  can_upload: boolean
  can_publish: boolean
  can_schedule: boolean
}

export interface ChannelConnection {
  id: string
  provider: 'youtube'
  remote_channel_id: string
  display_name: string
  state: ConnectionState
  capabilities: PublishingCapabilities
  created_at: string
  updated_at: string
}

export interface PublishArtifactSummary {
  id: string
  byte_size: number
  duration_ms: number
  width: number
  height: number
  created_at: string
}

export interface PublishAttempt {
  id: string
  project_id: string
  connection_id: string
  render_artifact_id: string
  request_id: string
  provider: 'youtube'
  state: PublishState
  remote_video_id?: string
  uploaded_bytes: number
  last_error_code?: string
  title: string
  description?: string
  scheduled_at?: string
  created_at: string
  updated_at: string
}

export interface CreatePublishAttemptPayload {
  connection_id: string
  render_artifact_id: string
  request_id: string
  title: string
  description: string
}

export async function listPublishingConnections(): Promise<ChannelConnection[]> {
  const response = await request<{ items: ChannelConnection[] }>('/api/v1/publishing/connections')
  return response.items
}

export async function startYouTubeOAuth(projectID: string): Promise<string> {
  const response = await request<{ authorization_url: string }>(`/api/v1/publishing/youtube/oauth/start?project_id=${encodeURIComponent(projectID)}`)
  return response.authorization_url
}

export async function listPublishArtifacts(projectID: string): Promise<PublishArtifactSummary[]> {
  const response = await request<{ items: PublishArtifactSummary[] }>(`/api/v1/projects/${encodeURIComponent(projectID)}/publishing/artifacts`)
  return response.items
}

export async function listPublishAttempts(projectID: string): Promise<PublishAttempt[]> {
  const response = await request<{ items: PublishAttempt[] }>(`/api/v1/projects/${encodeURIComponent(projectID)}/publishing/attempts`)
  return response.items
}

export async function createPublishAttempt(projectID: string, payload: CreatePublishAttemptPayload): Promise<PublishAttempt> {
  return request<PublishAttempt>(`/api/v1/projects/${encodeURIComponent(projectID)}/publishing/attempts`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function getPublishAttempt(projectID: string, attemptID: string): Promise<PublishAttempt> {
  return request<PublishAttempt>(`/api/v1/projects/${encodeURIComponent(projectID)}/publishing/attempts/${encodeURIComponent(attemptID)}`)
}

export async function retryPublishAttempt(projectID: string, attemptID: string): Promise<PublishAttempt> {
  return request<PublishAttempt>(`/api/v1/projects/${encodeURIComponent(projectID)}/publishing/attempts/${encodeURIComponent(attemptID)}/retry`, {
    method: 'POST',
  })
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const response = await apiFetch(url, {
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
