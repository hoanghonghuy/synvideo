import { ApiError } from '@/api/projects'

export type CreativeProposalStatus = 'draft' | 'approved' | 'superseded'

export interface CreativeProposalStructureItem {
  key: string
  title: string
  purpose: string
}

export interface CreativeProposalEditableContent {
  title_options: string[]
  hook_options: string[]
  audience_summary: string
  objective_summary: string
  narrative_angle: string
  estimated_duration_seconds: number | null
  format_rationale: string
  structure: CreativeProposalStructureItem[]
  visual_direction: string
  voice_direction: string
  music_direction: string
  caption_direction: string
  call_to_action: string
  research_gaps: string[]
  warnings: string[]
}

export interface CreativeProposalSummary {
  version: number
  revision: number
  status: CreativeProposalStatus
  source_brief_revision: number
  created_at: string
  updated_at: string
  approved_at: string | null
}

export interface CreativeProposal extends CreativeProposalSummary, CreativeProposalEditableContent {
  project_id: string
}

export interface PutCreativeProposalPayload extends CreativeProposalEditableContent {
  revision: number
}

export async function listCreativeProposals(projectId: string): Promise<CreativeProposalSummary[]> {
  return request<CreativeProposalSummary[]>(`/api/v1/projects/${projectId}/creative-proposals`)
}

export async function getCreativeProposal(projectId: string, version: number): Promise<CreativeProposal> {
  return request<CreativeProposal>(`/api/v1/projects/${projectId}/creative-proposals/${version}`)
}

export async function putCreativeProposal(
  projectId: string,
  version: number,
  payload: PutCreativeProposalPayload,
): Promise<CreativeProposal> {
  return request<CreativeProposal>(`/api/v1/projects/${projectId}/creative-proposals/${version}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

export async function approveCreativeProposal(
  projectId: string,
  version: number,
  revision: number,
): Promise<CreativeProposal> {
  return request<CreativeProposal>(`/api/v1/projects/${projectId}/creative-proposals/${version}/approve`, {
    method: 'POST',
    body: JSON.stringify({ revision }),
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
