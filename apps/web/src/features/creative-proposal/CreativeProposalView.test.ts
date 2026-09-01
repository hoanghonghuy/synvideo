import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
import type { CreativeProposal, CreativeProposalSummary } from './api'
import CreativeProposalView from './CreativeProposalView.vue'

const fetchMock = vi.fn()
const projectId = '11111111-1111-4111-8111-111111111111'

const project = {
  id: projectId,
  title: 'Video chi tiet',
  description: 'Mo ta du an',
  content_format: 'short',
  aspect_ratio: '9:16',
  target_duration_seconds: 60,
  locale: 'vi',
  status: 'active',
  created_at: '2026-08-31T08:00:00Z',
  updated_at: '2026-08-31T08:30:00Z',
}

const draftSummary: CreativeProposalSummary = {
  version: 2,
  revision: 4,
  status: 'draft',
  source_brief_revision: 3,
  created_at: '2026-08-31T09:00:00Z',
  updated_at: '2026-08-31T09:30:00Z',
  approved_at: null,
}

const approvedSummary: CreativeProposalSummary = {
  version: 1,
  revision: 2,
  status: 'approved',
  source_brief_revision: 2,
  created_at: '2026-08-31T08:00:00Z',
  updated_at: '2026-08-31T08:45:00Z',
  approved_at: '2026-08-31T08:50:00Z',
}

const draftProposal: CreativeProposal = {
  project_id: projectId,
  version: 2,
  revision: 4,
  status: 'draft',
  source_brief_revision: 3,
  title_options: ['Tieu de A', 'Tieu de B'],
  hook_options: ['Mo dau nhanh'],
  audience_summary: 'Nguoi xem tre',
  objective_summary: 'Tang nhan dien',
  narrative_angle: 'Ke cau chuyen thuc te',
  estimated_duration_seconds: 75,
  format_rationale: 'Phu hop video ngan',
  structure: [{ key: 'intro', title: 'Mo dau', purpose: 'Thu hut chu y' }],
  visual_direction: 'Canh quay sang',
  voice_direction: 'Giong am',
  music_direction: 'Nhac nhe',
  caption_direction: 'Phu de ngan',
  call_to_action: 'Theo doi kenh',
  research_gaps: ['Can so lieu moi'],
  warnings: ['Khong khang dinh qua muc'],
  created_at: '2026-08-31T09:00:00Z',
  updated_at: '2026-08-31T09:30:00Z',
  approved_at: null,
}

const approvedProposal: CreativeProposal = {
  ...draftProposal,
  version: 1,
  revision: 2,
  status: 'approved',
  source_brief_revision: 2,
  title_options: ['Tieu de da duyet'],
  approved_at: '2026-08-31T08:50:00Z',
}

const sampleProviders = {
  providers: [
    {
      id: 'lab-provider',
      display_name: 'Lab Provider',
      models: [
        { id: 'lab-model-v1', display_name: 'Lab Model V1' },
        { id: 'lab-model-v2', display_name: 'Lab Model V2' },
      ],
    },
  ],
}

type MockHandler = (url: string, init?: RequestInit) => Promise<{ ok: boolean; status: number; json: () => Promise<unknown> }> | { ok: boolean; status: number; json: () => Promise<unknown> }

let handlers: { method?: string; path: string; handler: MockHandler }[] = []

function mockRoute(method: string | undefined, path: string, response: unknown, status = 200) {
  handlers.push({
    method,
    path,
    handler: () => jsonResponse(response, status),
  })
}

function mockRouteFn(method: string | undefined, path: string, fn: MockHandler) {
  handlers.push({
    method,
    path,
    handler: fn,
  })
}

beforeEach(() => {
  handlers = []
  window.sessionStorage.clear()
  fetchMock.mockReset()
  fetchMock.mockImplementation((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    for (let i = handlers.length - 1; i >= 0; i--) {
      const h = handlers[i]
      if (h && (!h.method || h.method.toUpperCase() === method.toUpperCase()) && url === h.path) {
        return Promise.resolve(h.handler(url, init))
      }
    }
    if (url === '/api/v1/ai/text-generation-options') {
      return Promise.resolve(jsonResponse({ providers: [] }))
    }
    return Promise.resolve(jsonResponse({ error: { code: 'not_found' } }, 404))
  })
  vi.stubGlobal('fetch', fetchMock)
  i18n.global.locale.value = 'vi'
})

describe('CreativeProposalView', () => {
  it('renders an empty state when the proposal list is empty and no providers configured', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [])

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.text()).toContain('Chưa có AI Proposal')
    expect(wrapper.text()).toContain('Chưa có mô hình AI nào được cấu hình trong hệ thống.')
    expect(wrapper.find('[name="audience_summary"]').exists()).toBe(false)
  })

  it('renders a retryable error state when proposal list request fails after project loads', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    let failed = true
    mockRouteFn('GET', `/api/v1/projects/${projectId}/creative-proposals`, () => {
      if (failed) {
        return jsonResponse({ error: { code: 'request_failed' } }, 503)
      }
      return jsonResponse([draftSummary])
    })
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.text()).toContain('Không thể kết nối máy chủ.')
    expect(wrapper.text()).not.toContain('Chưa có AI Proposal')
    expect(wrapper.find('[data-testid="retry-proposal-list"]').exists()).toBe(true)

    failed = false
    await wrapper.find('[data-testid="retry-proposal-list"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Tieu de A')
    expect(wrapper.text()).not.toContain('Không thể kết nối máy chủ.')
  })

  it('renders newest version history with status labels and opens the newest proposal', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary, approvedSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.text()).toContain('Phiên bản 2')
    expect(wrapper.text()).toContain('Bản nháp')
    expect(wrapper.text()).toContain('Phiên bản 1')
    expect(wrapper.text()).toContain('Đã duyệt')
    expect(wrapper.text()).toContain('Tieu de A')
    expect(fetchMock).toHaveBeenCalledWith(
      `/api/v1/projects/${projectId}/creative-proposals/2`,
      expect.anything(),
    )
  })

  it('edits draft fields and sends PUT with the current revision', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('PUT', `/api/v1/projects/${projectId}/creative-proposals/2`, {
      ...draftProposal,
      revision: 5,
      audience_summary: 'Nguoi xem moi',
    })

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[name="audience_summary"]').setValue('Nguoi xem moi')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    const putCall = fetchMock.mock.calls.find(
      ([url, init]) =>
        url === `/api/v1/projects/${projectId}/creative-proposals/2` && init?.method === 'PUT',
    )
    expect(putCall).toBeDefined()
    expect(JSON.parse(String(putCall?.[1]?.body))).toMatchObject({
      revision: 4,
      audience_summary: 'Nguoi xem moi',
      title_options: ['Tieu de A', 'Tieu de B'],
    })
  })

  it('stores the returned revision and clears dirty state after a successful save', async () => {
    let currentProposal = { ...draftProposal }
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, currentProposal)
    mockRouteFn('PUT', `/api/v1/projects/${projectId}/creative-proposals/2`, (_url, init) => {
      const payload = JSON.parse(String(init?.body))
      currentProposal = { ...currentProposal, ...payload, revision: (payload.revision ?? 0) + 1 }
      return jsonResponse(currentProposal)
    })

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[name="audience_summary"]').setValue('Nguoi xem moi')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('Đã lưu AI Proposal.')
    expect(wrapper.text()).toContain('Bản sửa 5')

    await wrapper.find('[name="objective_summary"]').setValue('Muc tieu moi')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    const secondPut = fetchMock.mock.calls.filter(
      ([url, init]) =>
        url === `/api/v1/projects/${projectId}/creative-proposals/2` && init?.method === 'PUT',
    )[1]
    expect(JSON.parse(String(secondPut?.[1]?.body)).revision).toBe(5)
  })

  it('preserves values and dirty state after validation failure', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('PUT', `/api/v1/projects/${projectId}/creative-proposals/2`, {
      error: {
        code: 'validation_failed',
        message: 'Request validation failed.',
        fields: { audience_summary: 'required' },
      },
    }, 400)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[name="audience_summary"]').setValue('   ')
    await wrapper.find('[name="objective_summary"]').setValue('Muc tieu giu lai')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect((wrapper.find('[name="audience_summary"]').element as HTMLTextAreaElement).value).toBe('   ')
    expect((wrapper.find('[name="objective_summary"]').element as HTMLTextAreaElement).value).toBe('Muc tieu giu lai')
    expect(wrapper.text()).toContain('Vui lòng kiểm tra lại AI Proposal.')
    expect(wrapper.text()).toContain('Có thay đổi chưa lưu')
    expect(wrapper.find('[data-testid="retry-proposal-load"]').exists()).toBe(false)
  })

  it('shows stale save conflict without auto-overwrite', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('PUT', `/api/v1/projects/${projectId}/creative-proposals/2`, {
      error: {
        code: 'STALE_REVISION',
        message: 'Creative proposal revision is stale.',
      },
    }, 409)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[name="audience_summary"]').setValue('Thay doi xung dot')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('Proposal trên máy chủ đã thay đổi')
    expect((wrapper.find('[name="audience_summary"]').element as HTMLTextAreaElement).value).toBe('Thay doi xung dot')
  })

  it('does not expose a load retry after a failed mutation following a failed version switch', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary, approvedSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/1`, { error: { code: 'request_failed' } }, 503)
    mockRoute('PUT', `/api/v1/projects/${projectId}/creative-proposals/2`, { error: { code: 'validation_failed' } }, 422)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[data-testid="version-1"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="retry-proposal-load"]').exists()).toBe(true)

    await wrapper.find('[name="audience_summary"]').setValue('Thay doi truoc khi luu')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect((wrapper.find('[name="audience_summary"]').element as HTMLTextAreaElement).value).toBe('Thay doi truoc khi luu')
    expect(wrapper.text()).toContain('Vui lòng kiểm tra lại AI Proposal.')
    expect(wrapper.text()).toContain('Có thay đổi chưa lưu')
    expect(wrapper.find('[data-testid="retry-proposal-load"]').exists()).toBe(false)
  })

  it('preserves dirty edits when stale recovery fails to reload the latest version', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary, approvedSummary])
    let proposal2Calls = 0
    mockRouteFn('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, () => {
      proposal2Calls++
      if (proposal2Calls === 1) {
        return jsonResponse(draftProposal)
      }
      return jsonResponse({ error: { code: 'request_failed' } }, 503)
    })
    mockRoute('PUT', `/api/v1/projects/${projectId}/creative-proposals/2`, {
      error: { code: 'STALE_REVISION', message: 'Creative proposal revision is stale.' },
    }, 409)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[name="audience_summary"]').setValue('Thay doi can bao ve')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    await wrapper.find('[data-testid="reload-latest-proposal"]').trigger('click')
    await flushPromises()

    expect((wrapper.find('[name="audience_summary"]').element as HTMLTextAreaElement).value).toBe('Thay doi can bao ve')
    expect(wrapper.text()).toContain('Có thay đổi chưa lưu.')
    expect(wrapper.text()).toContain('Không thể kết nối máy chủ.')

    await wrapper.find('[data-testid="version-1"]').trigger('click')
    expect(wrapper.text()).toContain('Chọn Hủy thay đổi')
  })

  it('shows a visible retry state when the initial proposal version cannot be loaded', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, { error: { code: 'request_failed' } }, 503)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.text()).toContain('Không thể kết nối máy chủ.')
    expect(wrapper.find('[data-testid="retry-proposal-load"]').exists()).toBe(true)
    expect(wrapper.find('[name="audience_summary"]').exists()).toBe(false)
  })

  it('renders approved and superseded versions as read-only', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [approvedSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/1`, approvedProposal)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.text()).toContain('Phiên bản này chỉ đọc')
    expect(wrapper.find('[name="audience_summary"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="approve-proposal"]').exists()).toBe(false)
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()
  })

  it('explicitly approves a draft with the current revision and renders approved state after success', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('POST', `/api/v1/projects/${projectId}/creative-proposals/2/approve`, {
      ...draftProposal,
      status: 'approved',
      approved_at: '2026-08-31T10:00:00Z',
    })

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[data-testid="approve-proposal"]').trigger('click')
    expect(wrapper.text()).toContain('Xác nhận duyệt phiên bản này?')

    await wrapper.find('[data-testid="confirm-approve"]').trigger('click')
    await flushPromises()

    const approveCall = fetchMock.mock.calls.find(
      ([url, init]) =>
        url === `/api/v1/projects/${projectId}/creative-proposals/2/approve` && init?.method === 'POST',
    )
    expect(JSON.parse(String(approveCall?.[1]?.body))).toEqual({ revision: 4 })
    expect(wrapper.text()).toContain('Đã duyệt')
    expect(wrapper.text()).toContain('Phiên bản này chỉ đọc')
  })

  it('does not silently discard dirty edits when switching versions', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary, approvedSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/1`, approvedProposal)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[name="audience_summary"]').setValue('Dang sua')
    await wrapper.find('[data-testid="version-1"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Chọn Hủy thay đổi')
    expect((wrapper.find('[name="audience_summary"]').element as HTMLTextAreaElement).value).toBe('Dang sua')

    await wrapper.find('[data-testid="discard-version-switch"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Tieu de da duyet')
  })

  it('populates provider and model dropdowns and allows generating new proposal when empty', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [])
    mockRoute('GET', '/api/v1/ai/text-generation-options', sampleProviders)

    const jobId = '99999999-9999-4999-8999-999999999999'
    mockRoute('POST', `/api/v1/projects/${projectId}/creative-proposal-generations`, {
      id: jobId,
      state: 'queued',
      attempt: 0,
      max_attempts: 3,
      error_code: null,
      proposal_version: null,
      created_at: '2026-08-31T09:00:00Z',
      updated_at: '2026-08-31T09:00:00Z',
      started_at: null,
      finished_at: null,
    }, 202)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.find('[data-testid="select-provider"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="select-model"]').exists()).toBe(true)
    const generateBtn = wrapper.find('[data-testid="generate-proposal-btn"]')
    expect(generateBtn.exists()).toBe(true)
    expect(generateBtn.text()).toBe('Tạo AI Proposal')

    await generateBtn.trigger('click')
    await flushPromises()

    const postCall = fetchMock.mock.calls.find(
      ([url, init]) =>
        url === `/api/v1/projects/${projectId}/creative-proposal-generations` && init?.method === 'POST',
    )
    expect(postCall).toBeDefined()
    const payload = JSON.parse(String(postCall?.[1]?.body))
    expect(payload.provider_id).toBe('lab-provider')
    expect(payload.model_id).toBe('lab-model-v1')
    expect(payload.request_id).toBeDefined()
  })

  it('shows Regenerate button when proposals already exist and blocks when dirty', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('GET', '/api/v1/ai/text-generation-options', sampleProviders)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    const genBtn = wrapper.find('[data-testid="generate-proposal-btn"]')
    expect(genBtn.exists()).toBe(true)
    expect(genBtn.text()).toBe('Tạo lại AI Proposal')
    expect(genBtn.attributes('disabled')).toBeUndefined()

    // Make dirty
    await wrapper.find('[name="audience_summary"]').setValue('Thay doi nhap')
    await flushPromises()

    expect(wrapper.find('[data-testid="dirty-generation-blocked"]').exists()).toBe(true)
    expect(genBtn.attributes('disabled')).toBeDefined()
  })

  it('tracks job progress through polling and loads new version on succeeded', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('GET', '/api/v1/ai/text-generation-options', sampleProviders)

    const jobId = '88888888-8888-4888-8888-888888888888'
    mockRoute('POST', `/api/v1/projects/${projectId}/creative-proposal-generations`, {
      id: jobId,
      state: 'queued',
      attempt: 0,
      max_attempts: 3,
      error_code: null,
      proposal_version: null,
      created_at: '2026-08-31T09:00:00Z',
      updated_at: '2026-08-31T09:00:00Z',
      started_at: null,
      finished_at: null,
    }, 202)

    let pollCount = 0
    mockRouteFn('GET', `/api/v1/projects/${projectId}/creative-proposal-generations/${jobId}`, () => {
      pollCount++
      if (pollCount === 1) {
        return jsonResponse({
          id: jobId,
          state: 'running',
          attempt: 1,
          max_attempts: 3,
          error_code: null,
          proposal_version: null,
          created_at: '2026-08-31T09:00:00Z',
          updated_at: '2026-08-31T09:00:00Z',
          started_at: '2026-08-31T09:00:01Z',
          finished_at: null,
        })
      }
      return jsonResponse({
        id: jobId,
        state: 'succeeded',
        attempt: 1,
        max_attempts: 3,
        error_code: null,
        proposal_version: 3,
        created_at: '2026-08-31T09:00:00Z',
        updated_at: '2026-08-31T09:00:05Z',
        started_at: '2026-08-31T09:00:01Z',
        finished_at: '2026-08-31T09:00:05Z',
      })
    })

    const v3Summary = {
      version: 3,
      revision: 1,
      status: 'draft',
      source_brief_revision: 3,
      created_at: '2026-08-31T09:00:05Z',
      updated_at: '2026-08-31T09:00:05Z',
      approved_at: null,
    }
    const v3Proposal = {
      ...draftProposal,
      version: 3,
      revision: 1,
      title_options: ['Tieu de v3 moi'],
    }

    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/3`, v3Proposal)

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      await wrapper.find('[data-testid="generate-proposal-btn"]').trigger('click')
      await flushPromises()

      expect(wrapper.find('[data-testid="job-progress-banner"]').exists()).toBe(true)

      // First poll -> running
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      expect(wrapper.find('[data-testid="job-progress-banner"]').text()).toContain('Đang tạo AI Proposal...')

      // Update summaries route for refresh
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [v3Summary, draftSummary])

      // Second poll -> succeeded
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()

      expect(wrapper.find('[data-testid="job-progress-banner"]').exists()).toBe(false)
      expect(wrapper.text()).toContain('Tieu de v3 moi')
      expect(wrapper.text()).toContain('Phiên bản 3')
    } finally {
      vi.useRealTimers()
    }
  })

  it('displays localized error banner when generation fails while keeping current proposal mounted', async () => {
    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('GET', '/api/v1/ai/text-generation-options', sampleProviders)

    const jobId = '77777777-7777-4777-8777-777777777777'
    mockRoute('POST', `/api/v1/projects/${projectId}/creative-proposal-generations`, {
      id: jobId,
      state: 'running',
      attempt: 1,
      max_attempts: 3,
      error_code: null,
      proposal_version: null,
      created_at: '2026-08-31T09:00:00Z',
      updated_at: '2026-08-31T09:00:00Z',
      started_at: '2026-08-31T09:00:01Z',
      finished_at: null,
    }, 202)

    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposal-generations/${jobId}`, {
      id: jobId,
      state: 'failed',
      attempt: 3,
      max_attempts: 3,
      error_code: 'CREATIVE_BRIEF_REQUIRED',
      proposal_version: null,
      created_at: '2026-08-31T09:00:00Z',
      updated_at: '2026-08-31T09:00:05Z',
      started_at: '2026-08-31T09:00:01Z',
      finished_at: '2026-08-31T09:00:05Z',
    })

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      await wrapper.find('[data-testid="generate-proposal-btn"]').trigger('click')
      await flushPromises()

      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()

      expect(wrapper.find('[data-testid="job-error-banner"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="job-error-banner"]').text()).toContain('Cần có Creative Brief trước khi tạo AI Proposal.')
      expect(wrapper.find('[data-testid="retry-generation-btn"]').exists()).toBe(true)
      expect(wrapper.text()).toContain('Tieu de A')
    } finally {
      vi.useRealTimers()
    }
  })

  it('resumes tracking active generation job from sessionStorage on page mount', async () => {
    const activeJobId = '66666666-6666-4666-8666-666666666666'
    window.sessionStorage.setItem(`proposal_job_${projectId}`, activeJobId)

    mockRoute('GET', `/api/v1/projects/${projectId}`, project)
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
    mockRoute('GET', '/api/v1/ai/text-generation-options', sampleProviders)

    mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposal-generations/${activeJobId}`, {
      id: activeJobId,
      state: 'running',
      attempt: 1,
      max_attempts: 3,
      error_code: null,
      proposal_version: null,
      created_at: '2026-08-31T09:00:00Z',
      updated_at: '2026-08-31T09:00:00Z',
      started_at: '2026-08-31T09:00:01Z',
      finished_at: null,
    })

    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      const wrapper = await mountCreativeProposalView()
      await flushPromises()

      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()

      expect(wrapper.find('[data-testid="job-progress-banner"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="job-progress-banner"]').text()).toContain('Đang tạo AI Proposal...')
    } finally {
      vi.useRealTimers()
    }
  })

  it('preserves succeeded generation job without re-enabling regenerate when loading the generated proposal fails transiently', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      mockRoute('GET', `/api/v1/projects/${projectId}`, project)
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
      mockRoute('GET', '/api/v1/ai/text-generation-options', sampleProviders)

      const jobId = '77777777-7777-4777-8777-777777777777'
      mockRoute('POST', `/api/v1/projects/${projectId}/creative-proposal-generations`, {
        id: jobId,
        state: 'queued',
        attempt: 0,
        max_attempts: 3,
        error_code: null,
        proposal_version: null,
        created_at: '2026-08-31T09:00:00Z',
        updated_at: '2026-08-31T09:00:00Z',
        started_at: null,
        finished_at: null,
      }, 202)

      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposal-generations/${jobId}`, {
        id: jobId,
        state: 'succeeded',
        attempt: 1,
        max_attempts: 3,
        error_code: null,
        proposal_version: 3,
        created_at: '2026-08-31T09:00:00Z',
        updated_at: '2026-08-31T09:01:00Z',
        started_at: '2026-08-31T09:00:10Z',
        finished_at: '2026-08-31T09:01:00Z',
      })

      // Simulate failure when fetching proposals / version 3
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/3`, { error: { code: 'request_failed' } }, 503)

      const wrapper = await mountCreativeProposalView()
      await flushPromises()

      await wrapper.find('[data-testid="generate-proposal-btn"]').trigger('click')
      await flushPromises()

      // Poll triggers succeeded -> loadVersion fails with 503
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      await flushPromises()

      // Should show succeeded banner with load retry, and regenerate button must be disabled
      expect(wrapper.find('[data-testid="job-succeeded-load-failed-banner"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="retry-load-generated-btn"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="generate-proposal-btn"]').attributes('disabled')).toBeDefined()

      // Now fix route and click retry load
      const v3Summary: CreativeProposalSummary = {
        ...draftSummary,
        version: 3,
        revision: 1,
      }
      const v3Proposal: CreativeProposal = {
        ...draftProposal,
        version: 3,
        revision: 1,
        title_options: ['Tieu de v3 sau retry load'],
      }
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [v3Summary, draftSummary])
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/3`, v3Proposal)

      await wrapper.find('[data-testid="retry-load-generated-btn"]').trigger('click')
      await flushPromises()
      await flushPromises()

      expect(wrapper.find('[data-testid="job-succeeded-load-failed-banner"]').exists()).toBe(false)
      expect(wrapper.text()).toContain('Tieu de v3 sau retry load')
    } finally {
      vi.useRealTimers()
    }
  })

  it('recovers from transient status polling failure by continuing to poll the same job ID without creating a new job', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      mockRoute('GET', `/api/v1/projects/${projectId}`, project)
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [draftSummary])
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, draftProposal)
      mockRoute('GET', '/api/v1/ai/text-generation-options', sampleProviders)

      const jobId = '66666666-7777-4666-8777-666666666666'
      mockRoute('POST', `/api/v1/projects/${projectId}/creative-proposal-generations`, {
        id: jobId,
        state: 'queued',
        attempt: 0,
        max_attempts: 3,
        error_code: null,
        proposal_version: null,
        created_at: '2026-08-31T09:00:00Z',
        updated_at: '2026-08-31T09:00:00Z',
        started_at: null,
        finished_at: null,
      }, 202)

      let pollCount = 0
      mockRouteFn('GET', `/api/v1/projects/${projectId}/creative-proposal-generations/${jobId}`, () => {
        pollCount++
        if (pollCount === 1) {
          // Poll 1 fails with 503
          return jsonResponse({ error: { code: 'request_failed' } }, 503)
        }
        // Poll 2 succeeds with succeeded state
        return jsonResponse({
          id: jobId,
          state: 'succeeded',
          attempt: 1,
          max_attempts: 3,
          error_code: null,
          proposal_version: 3,
          created_at: '2026-08-31T09:00:00Z',
          updated_at: '2026-08-31T09:01:00Z',
          started_at: '2026-08-31T09:00:10Z',
          finished_at: '2026-08-31T09:01:00Z',
        })
      })

      const v3Summary: CreativeProposalSummary = {
        ...draftSummary,
        version: 3,
        revision: 1,
      }
      const v3Proposal: CreativeProposal = {
        ...draftProposal,
        version: 3,
        revision: 1,
        title_options: ['Tieu de v3 sau polling recovery'],
      }
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals`, [v3Summary, draftSummary])
      mockRoute('GET', `/api/v1/projects/${projectId}/creative-proposals/3`, v3Proposal)

      const wrapper = await mountCreativeProposalView()
      await flushPromises()

      await wrapper.find('[data-testid="generate-proposal-btn"]').trigger('click')
      await flushPromises()

      // Poll 1 fires -> fails with 503
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      await flushPromises()

      expect(pollCount).toBe(1)
      expect(wrapper.find('[data-testid="job-error-banner"]').exists()).toBe(true)

      // Poll 2 fires automatically after 1s -> succeeds with succeeded state
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      await flushPromises()

      expect(pollCount).toBe(2)
      expect(wrapper.find('[data-testid="job-error-banner"]').exists()).toBe(false)
      expect(wrapper.text()).toContain('Tieu de v3 sau polling recovery')

      // Assert only 1 POST was ever made (no new job or request_id created)
      const postCalls = fetchMock.mock.calls.filter(
        ([url, init]) =>
          url === `/api/v1/projects/${projectId}/creative-proposal-generations` && init?.method === 'POST',
      )
      expect(postCalls).toHaveLength(1)
    } finally {
      vi.useRealTimers()
    }
  })
})

async function mountCreativeProposalView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/projects/:id', component: { template: '<div />' } },
      { path: '/projects/:id/creative-proposal', component: CreativeProposalView },
    ],
  })
  router.push(`/projects/${projectId}/creative-proposal`)
  await router.isReady()

  return mount(CreativeProposalView, {
    global: {
      plugins: [i18n, router],
    },
  })
}

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  }
}
