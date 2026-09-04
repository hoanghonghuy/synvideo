export interface ImageGenerationOptionModel {
  id: string
  display_name: string
}

export interface ImageGenerationOptionProvider {
  id: string
  display_name: string
  models: ImageGenerationOptionModel[]
}

export interface ImageGenerationOptionsResponse {
  providers: ImageGenerationOptionProvider[]
}

export interface CreateSceneImageGenerationInput {
  request_id: string
  provider_id: string
  model_id: string
  prompt: string
  assign_primary_visual: boolean
}

export interface SceneImageGenerationJobView {
  id: string
  state: 'queued' | 'running' | 'succeeded' | 'failed'
  attempt: number
  max_attempts: number
  error_code?: string
  media_asset_id?: string
  assigned_primary_visual: boolean
  created_at: string
  updated_at: string
}

interface ApiErrorPayload {
  error?: {
    code?: string
    message?: string
    fields?: Record<string, string>
  }
}

export class GeneratedImageApiError extends Error {
  code: string
  status: number
  fields?: Record<string, string>

  constructor(status: number, code: string, message: string, fields?: Record<string, string>) {
    super(message)
    this.name = 'GeneratedImageApiError'
    this.status = status
    this.code = code
    this.fields = fields
  }
}

async function handleResponse<T>(response: Response): Promise<T> {
  const data = (await response.json().catch(() => ({}))) as T & ApiErrorPayload
  if (!response.ok) {
    const error = (data as ApiErrorPayload).error
    throw new GeneratedImageApiError(
      response.status,
      error?.code ?? 'UNKNOWN_ERROR',
      error?.message ?? `Request failed with status ${response.status}`,
      error?.fields,
    )
  }
  return data
}

export async function fetchImageGenerationOptions(): Promise<ImageGenerationOptionsResponse> {
  const response = await fetch('/api/v1/ai/image-generation-options', {
    headers: { Accept: 'application/json' },
  })
  return handleResponse<ImageGenerationOptionsResponse>(response)
}

export async function createSceneImageGeneration(
  projectID: string,
  version: number,
  sceneKey: string,
  input: CreateSceneImageGenerationInput,
): Promise<SceneImageGenerationJobView> {
  const response = await fetch(
    `/api/v1/projects/${encodeURIComponent(projectID)}/scene-plans/${encodeURIComponent(version)}/scenes/${encodeURIComponent(sceneKey)}/image-generations`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
  return handleResponse<SceneImageGenerationJobView>(response)
}

export async function getSceneImageGeneration(
  projectID: string,
  jobID: string,
): Promise<SceneImageGenerationJobView> {
  const response = await fetch(
    `/api/v1/projects/${encodeURIComponent(projectID)}/image-generations/${encodeURIComponent(jobID)}`,
    { headers: { Accept: 'application/json' } },
  )
  return handleResponse<SceneImageGenerationJobView>(response)
}
