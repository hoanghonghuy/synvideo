export type Capability = 'text' | 'image' | 'tts'

export interface ModelSettingView {
  id: string
  display_name: string
  enabled: boolean
  capabilities?: Capability[]
}

export interface VoiceSettingView {
  id: string
  display_name: string
  enabled: boolean
}

export interface ProviderSettingView {
  id: string
  display_name: string
  configured: boolean
  enabled: boolean
  has_api_key: boolean
  revision: number
  models: ModelSettingView[]
  voices: VoiceSettingView[]
}

export interface ProviderSettingsListResponse {
  providers: ProviderSettingView[]
}

export interface PutSettingInput {
  revision?: number
  enabled: boolean
  enabled_text_model_ids: string[]
  enabled_image_model_ids: string[]
  enabled_voice_ids: string[]
  api_key?: string
}

export interface ApiErrorPayload {
  error?: {
    code: string
    message: string
    fields?: Record<string, string>
  }
}

export class ProviderApiError extends Error {
  code: string
  status: number
  fields?: Record<string, string>

  constructor(status: number, code: string, message: string, fields?: Record<string, string>) {
    super(message)
    this.name = 'ProviderApiError'
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
    throw new ProviderApiError(
      res.status,
      error?.code || 'UNKNOWN_ERROR',
      error?.message || `Request failed with status ${res.status}`,
      error?.fields,
    )
  }

  return data
}

export async function fetchProviderSettings(): Promise<ProviderSettingsListResponse> {
  const res = await fetch('/api/v1/ai/provider-settings', {
    headers: {
      Accept: 'application/json',
    },
  })
  return handleResponse<ProviderSettingsListResponse>(res)
}

export async function saveProviderSetting(
  providerId: string,
  input: PutSettingInput,
): Promise<ProviderSettingView> {
  const res = await fetch(`/api/v1/ai/provider-settings/${encodeURIComponent(providerId)}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify(input),
  })
  return handleResponse<ProviderSettingView>(res)
}

export async function deleteProviderSetting(
  providerId: string,
  revision: number,
): Promise<void> {
  const res = await fetch(
    `/api/v1/ai/provider-settings/${encodeURIComponent(providerId)}?revision=${encodeURIComponent(
      revision,
    )}`,
    {
      method: 'DELETE',
      headers: {
        Accept: 'application/json',
      },
    },
  )
  return handleResponse<void>(res)
}
