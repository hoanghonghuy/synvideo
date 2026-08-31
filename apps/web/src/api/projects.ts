export type ContentFormat = 'short' | 'long' | 'flexible'
export type AspectRatio = '16:9' | '9:16' | '1:1' | '4:5'
export type ProjectLocale = 'vi' | 'en'
export type ProjectStatus = 'active' | 'archived'

export interface Project {
  id: string
  title: string
  description: string
  content_format: ContentFormat
  aspect_ratio: AspectRatio
  target_duration_seconds: number | null
  locale: ProjectLocale
  status: ProjectStatus
  created_at: string
  updated_at: string
}

export interface ProjectPayload {
  title: string
  description: string
  content_format: ContentFormat
  aspect_ratio: AspectRatio
  target_duration_seconds: number | null
  locale: ProjectLocale
}

export interface UpdateProjectPayload extends Partial<ProjectPayload> {
  status?: ProjectStatus
}

export interface ProjectListResponse {
  projects: Project[]
  next_cursor?: string
}

export class ApiError extends Error {
  code: string
  fields: Record<string, string>
  status: number

  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields
  }
}

export async function listProjects(cursor = '', limit = 20): Promise<ProjectListResponse> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (cursor) {
    params.set('cursor', cursor)
  }
  return request<ProjectListResponse>(`/api/v1/projects?${params.toString()}`)
}

export async function createProject(payload: ProjectPayload): Promise<Project> {
  return request<Project>('/api/v1/projects', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function getProject(id: string): Promise<Project> {
  return request<Project>(`/api/v1/projects/${id}`)
}

export async function updateProject(id: string, payload: UpdateProjectPayload): Promise<Project> {
  return request<Project>(`/api/v1/projects/${id}`, {
    method: 'PATCH',
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
    throw new ApiError(
      response.status,
      error?.code ?? 'request_failed',
      error?.message ?? 'Request failed.',
      error?.fields ?? {},
    )
  }

  return body as T
}
