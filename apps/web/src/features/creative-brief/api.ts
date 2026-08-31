import { ApiError } from '@/api/projects'

export type DistributionTarget = 'youtube' | 'tiktok' | 'instagram' | 'other'

export interface CreativeBrief {
  project_id: string
  revision: number
  source_text: string
  target_audience: string
  objective: string
  desired_style: string
  tone: string
  distribution_targets: DistributionTarget[]
  call_to_action: string
  must_include: string[]
  must_avoid: string[]
  created_at: string
  updated_at: string
}

export interface CreativeBriefPayload {
  source_text: string
  target_audience: string
  objective: string
  desired_style: string
  tone: string
  distribution_targets: DistributionTarget[]
  call_to_action: string
  must_include: string[]
  must_avoid: string[]
}

export interface UpdateCreativeBriefPayload extends CreativeBriefPayload {
  revision: number
}

export class CreativeBriefNotFoundError extends Error {
  constructor() {
    super('Creative brief was not found.')
    this.name = 'CreativeBriefNotFoundError'
  }
}

export async function getCreativeBrief(projectId: string): Promise<CreativeBrief> {
  return request<CreativeBrief>(`/api/v1/projects/${projectId}/creative-brief`)
}

export async function putCreativeBrief(
  projectId: string,
  payload: CreativeBriefPayload | UpdateCreativeBriefPayload,
): Promise<CreativeBrief> {
  return request<CreativeBrief>(`/api/v1/projects/${projectId}/creative-brief`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
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
    if (response.status === 404 && error?.code === 'creative_brief_not_found') {
      throw new CreativeBriefNotFoundError()
    }
    throw new ApiError(
      response.status,
      error?.code ?? 'request_failed',
      error?.message ?? 'Request failed.',
      error?.fields ?? {},
    )
  }

  return body as T
}
