import { ApiError } from '@/api/projects'

export type AudioMixState = 'CURRENT' | 'STALE' | 'BROKEN' | 'ERROR'
export type AudioMixLoopPolicy = 'NO_LOOP' | 'LOOP_TO_TARGET'

export interface AudioMixDucking {
  enabled: boolean
  reduction_db: number
  attack_ms: number
  release_ms: number
}

export interface AudioMixConfig {
  music_trim_start_ms: number
  start_offset_ms: number
  loop_policy: AudioMixLoopPolicy
  music_gain_db: number
  narration_gain_db: number
  ducking: AudioMixDucking
}

export interface AudioMixDocument {
  id: string
  project_id: string
  revision: number
  scene_plan_version: number
  music_asset_id: string
  music_duration_ms: number
  narration_lineage_id: string
  narration_duration_ms: number
  config: AudioMixConfig
  created_at: string
  updated_at: string
}

export interface AudioMixView extends AudioMixDocument {
  state: AudioMixState
}

export interface AudioMixSnapshot {
  document_id: string
  revision: number
  project_id: string
  scene_plan_version: number
  music_asset_id: string
  music_duration_ms: number
  narration_lineage_id: string
  narration_duration_ms: number
  config: AudioMixConfig
}

const base = (projectID: string) => `/api/v1/projects/${encodeURIComponent(projectID)}/audio-mix`

export async function getAudioMix(projectID: string): Promise<AudioMixView> {
  return request<AudioMixView>(base(projectID))
}

export async function createAudioMix(projectID: string, input: { music_asset_id: string; config: AudioMixConfig }): Promise<AudioMixView> {
  return request<AudioMixView>(base(projectID), { method: 'POST', body: JSON.stringify(input) })
}

export async function updateAudioMix(projectID: string, input: { expected_revision: number; music_asset_id: string; config: AudioMixConfig }): Promise<AudioMixView> {
  return request<AudioMixView>(base(projectID), { method: 'PUT', body: JSON.stringify(input) })
}

export async function rebindAudioMixNarration(projectID: string, expectedRevision: number): Promise<AudioMixView> {
  return request<AudioMixView>(`${base(projectID)}/rebind-narration`, {
    method: 'POST',
    body: JSON.stringify({ expected_revision: expectedRevision }),
  })
}

export async function listAudioMixHistory(projectID: string): Promise<AudioMixDocument[]> {
  return request<AudioMixDocument[]>(`${base(projectID)}/history`)
}

export async function getAudioMixSnapshot(projectID: string): Promise<AudioMixSnapshot> {
  return request<AudioMixSnapshot>(`${base(projectID)}/snapshot`)
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: { 'Content-Type': 'application/json', Accept: 'application/json', ...init.headers },
  })
  const body = await response.json().catch(() => null)
  if (!response.ok) {
    const error = isErrorEnvelope(body) ? body.error : undefined
    throw new ApiError(
      response.status,
      error?.code ?? 'request_failed',
      error?.message ?? 'Request failed.',
      error?.fields ?? {},
    )
  }
  return body as T
}

function isErrorEnvelope(value: unknown): value is { error?: { code?: string; message?: string; fields?: Record<string, string> } } {
  return typeof value === 'object' && value !== null && 'error' in value
}
