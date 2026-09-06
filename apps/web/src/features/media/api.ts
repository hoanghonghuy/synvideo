import { ApiError } from '@/api/projects'

export type MediaAssetKind = 'image' | 'video' | 'audio' | 'document' | 'other'
export type MediaAssetOrigin =
  | 'upload'
  | 'creator_media'
  | 'stock'
  | 'generated_image'
  | 'generated_video'
  | 'generated_audio'
  | 'system'

export type StockMediaKind = 'image' | 'video'
export type StockMediaOrientation = '' | 'landscape' | 'portrait' | 'square'

export interface MediaAsset {
  id: string
  project_id: string
  kind: MediaAssetKind
  origin: MediaAssetOrigin
  mime_type: string
  byte_size: number
  sha256: string
  original_filename?: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface SceneMediaBinding {
  id?: string
  scene_plan_version?: number
  scene_key: string
  role: 'primary_visual'
  binding_version?: number
  asset_id?: string
  status?: 'active' | 'superseded'
  created_at?: string
  superseded_at?: string | null
}

export interface SceneMediaEntry {
  scene_key: string
  role: 'primary_visual'
  binding?: SceneMediaBinding
  asset?: MediaAsset
}

export interface StockMediaResult {
  provider_key: string
  provider_result_id: string
  kind: StockMediaKind
  preview_url: string
  source_page_url: string
  creator_name: string
  creator_url: string
  license_summary: string
  license_reference: string
  attribution_text: string
  acquirable: boolean
}

export interface StockMediaSearchPage {
  results: StockMediaResult[] | null
  page: number
  per_page: number
  has_next_page: boolean
}

export interface StockMediaAcquisition {
  Asset: MediaAsset
  Reused: boolean
}

export interface StockMediaSearchInput {
  provider: string
  query: string
  kind: StockMediaKind
  orientation?: StockMediaOrientation
  page?: number
  perPage?: number
}

export interface UploadOptions {
  signal?: AbortSignal
  onProgress?: (percentage: number) => void
}

const API_PREFIX = '/api/v1'

export function listMediaAssets(projectId: string): Promise<{ assets: MediaAsset[] }> {
  return request<{ assets: MediaAsset[] }>(`${API_PREFIX}/projects/${projectId}/media-assets`)
}

export function deleteMediaAsset(projectId: string, assetId: string): Promise<void> {
  return request<void>(`${API_PREFIX}/projects/${projectId}/media-assets/${assetId}`, {
    method: 'DELETE',
  })
}

export function searchStockMedia(
  projectId: string,
  input: StockMediaSearchInput,
): Promise<StockMediaSearchPage> {
  const params = new URLSearchParams({
    provider: input.provider,
    q: input.query,
    kind: input.kind,
    page: String(input.page ?? 1),
    per_page: String(input.perPage ?? 20),
  })
  if (input.orientation) params.set('orientation', input.orientation)
  return request<StockMediaSearchPage>(
    `${API_PREFIX}/projects/${projectId}/stock-media/search?${params.toString()}`,
  )
}

export function acquireStockMedia(
  projectId: string,
  result: Pick<StockMediaResult, 'provider_key' | 'provider_result_id' | 'kind'>,
): Promise<StockMediaAcquisition> {
  return request<StockMediaAcquisition>(`${API_PREFIX}/projects/${projectId}/stock-media/acquisitions`, {
    method: 'POST',
    body: JSON.stringify({
      provider_key: result.provider_key,
      provider_result_id: result.provider_result_id,
      kind: result.kind,
    }),
  })
}

export function listSceneMediaBindings(projectId: string, version: number): Promise<SceneMediaEntry[]> {
  return request<SceneMediaEntry[]>(
    `${API_PREFIX}/projects/${projectId}/scene-plans/${version}/media-bindings`,
  )
}

export function assignPrimaryVisual(
  projectId: string,
  version: number,
  sceneKey: string,
  assetId: string,
): Promise<SceneMediaEntry> {
  return request<SceneMediaEntry>(
    `${API_PREFIX}/projects/${projectId}/scene-plans/${version}/scenes/${encodeURIComponent(sceneKey)}/primary-visual`,
    {
      method: 'PUT',
      body: JSON.stringify({ asset_id: assetId }),
    },
  )
}

export function listPrimaryVisualHistory(
  projectId: string,
  version: number,
  sceneKey: string,
): Promise<SceneMediaEntry[]> {
  return request<SceneMediaEntry[]>(
    `${API_PREFIX}/projects/${projectId}/scene-plans/${version}/scenes/${encodeURIComponent(sceneKey)}/primary-visual/history`,
  )
}

export function mediaAssetContentURL(projectId: string, assetId: string): string {
  return `${API_PREFIX}/projects/${projectId}/media-assets/${assetId}/content`
}

export function uploadMediaAsset(
  projectId: string,
  file: File,
  options: UploadOptions = {},
): Promise<MediaAsset> {
  if (typeof XMLHttpRequest === 'undefined') {
    return request<MediaAsset>(`${API_PREFIX}/projects/${projectId}/media-assets`, {
      method: 'POST',
      body: createUploadBody(file),
      signal: options.signal,
    })
  }

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const abort = () => xhr.abort()
    const cleanup = () => options.signal?.removeEventListener('abort', abort)
    xhr.open('POST', `${API_PREFIX}/projects/${projectId}/media-assets`)
    xhr.responseType = 'json'
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) options.onProgress?.(Math.round((event.loaded / event.total) * 100))
    }
    xhr.onload = () => {
      const body = xhr.response ?? parseJSON(xhr.responseText)
      if (xhr.status >= 200 && xhr.status < 300) {
        cleanup()
        resolve(body as MediaAsset)
      } else {
        cleanup()
        reject(apiErrorFromResponse(xhr.status, body))
      }
    }
    xhr.onerror = () => {
      cleanup()
      reject(new ApiError(0, 'request_failed', 'Request failed.'))
    }
    xhr.onabort = () => {
      cleanup()
      reject(new DOMException('Upload cancelled.', 'AbortError'))
    }
    if (options.signal) {
      if (options.signal.aborted) {
        xhr.abort()
        return
      }
      options.signal.addEventListener('abort', abort, { once: true })
    }
    xhr.send(createUploadBody(file))
  })
}

function createUploadBody(file: File): FormData {
  const form = new FormData()
  form.append('file', file, file.name)
  return form
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: {
      ...(typeof FormData !== 'undefined' && init.body instanceof FormData
        ? {}
        : { 'Content-Type': 'application/json' }),
      ...init.headers,
    },
  })
  const body = await response.json().catch(() => null)
  if (!response.ok) throw apiErrorFromResponse(response.status, body)
  return body as T
}

function apiErrorFromResponse(status: number, body: unknown): ApiError {
  const error = isErrorEnvelope(body) ? body.error : undefined
  return new ApiError(
    status,
    error?.code ?? 'request_failed',
    error?.message ?? 'Request failed.',
    error?.fields ?? {},
  )
}

function isErrorEnvelope(value: unknown): value is { error?: { code?: string; message?: string; fields?: Record<string, string> } } {
  return typeof value === 'object' && value !== null && 'error' in value
}

function parseJSON(value: string): unknown {
  try {
    return JSON.parse(value)
  } catch {
    return null
  }
}
