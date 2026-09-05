export type CaptionState = 'CURRENT' | 'STALE' | 'REBUILDING' | 'ERROR'

export interface CaptionSegment {
  id: string
  text: string
  start_ms: number
  end_ms: number
}

export interface CaptionStyle {
  alignment: 'left' | 'center' | 'right'
  position: 'top' | 'middle' | 'bottom'
  font_family_token?: string
  size: 'small' | 'medium' | 'large'
  weight: 'normal' | 'semibold' | 'bold'
}

export interface CaptionDocument {
  id: string
  project_id: string
  scene_plan_version: number
  scene_key: string
  revision: number
  source_binding_id: string
  source_asset_id: string
  source_duration_ms: number
  segments: CaptionSegment[]
  style: CaptionStyle
  created_at: string
}

export interface CaptionView extends CaptionDocument {
  state: CaptionState
}

export interface CaptionSnapshot {
  document_id: string
  revision: number
  source_binding_id: string
  source_asset_id: string
  source_duration_ms: number
  segments: CaptionSegment[]
  style: CaptionStyle
}

export class CaptionApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields?: Record<string, string>,
  ) {
    super(message)
    this.name = 'CaptionApiError'
  }
}

async function handleResponse<T>(res: Response): Promise<T> {
  const data = (await res.json().catch(() => ({}))) as T & {
    error?: { code?: string; message?: string; fields?: Record<string, string> }
  }
  if (!res.ok) {
    throw new CaptionApiError(
      res.status,
      data.error?.code ?? 'UNKNOWN_ERROR',
      data.error?.message ?? `Request failed with status ${res.status}`,
      data.error?.fields,
    )
  }
  return data
}

function base(projectID: string, version: number, sceneKey: string): string {
  return `/api/v1/projects/${encodeURIComponent(projectID)}/scene-plans/${encodeURIComponent(version)}/scenes/${encodeURIComponent(sceneKey)}/captions`
}

export async function getCaptions(projectID: string, version: number, sceneKey: string): Promise<CaptionView> {
  return handleResponse(await fetch(base(projectID, version, sceneKey), { headers: { Accept: 'application/json' } }))
}

export async function deriveCaptions(projectID: string, version: number, sceneKey: string): Promise<CaptionView> {
  return handleResponse(await fetch(`${base(projectID, version, sceneKey)}/derive`, { method: 'POST', headers: { Accept: 'application/json' } }))
}

export async function updateCaptions(projectID: string, version: number, sceneKey: string, input: {
  expected_revision: number
  segments: CaptionSegment[]
  style: CaptionStyle
}): Promise<CaptionView> {
  return handleResponse(await fetch(base(projectID, version, sceneKey), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify(input),
  }))
}

export async function rebuildCaptions(projectID: string, version: number, sceneKey: string, expectedRevision: number): Promise<CaptionView> {
  return handleResponse(await fetch(`${base(projectID, version, sceneKey)}/rebuild`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify({ expected_revision: expectedRevision }),
  }))
}

export async function listCaptionHistory(projectID: string, version: number, sceneKey: string): Promise<CaptionDocument[]> {
  return handleResponse(await fetch(`${base(projectID, version, sceneKey)}/history`, { headers: { Accept: 'application/json' } }))
}

export async function getCaptionSnapshot(projectID: string, version: number, sceneKey: string): Promise<CaptionSnapshot> {
  return handleResponse(await fetch(`${base(projectID, version, sceneKey)}/snapshot`, { headers: { Accept: 'application/json' } }))
}
