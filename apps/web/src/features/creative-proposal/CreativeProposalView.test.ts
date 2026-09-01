import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
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

const draftSummary = {
  version: 2,
  revision: 4,
  status: 'draft',
  source_brief_revision: 3,
  created_at: '2026-08-31T09:00:00Z',
  updated_at: '2026-08-31T09:30:00Z',
  approved_at: null,
}

const approvedSummary = {
  version: 1,
  revision: 2,
  status: 'approved',
  source_brief_revision: 2,
  created_at: '2026-08-31T08:00:00Z',
  updated_at: '2026-08-31T08:45:00Z',
  approved_at: '2026-08-31T08:50:00Z',
}

const draftProposal = {
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

const approvedProposal = {
  ...draftProposal,
  version: 1,
  revision: 2,
  status: 'approved',
  source_brief_revision: 2,
  title_options: ['Tieu de da duyet'],
  approved_at: '2026-08-31T08:50:00Z',
}

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
  i18n.global.locale.value = 'vi'
})

describe('CreativeProposalView', () => {
  it('renders an empty state when the proposal list is empty', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(project)).mockResolvedValueOnce(jsonResponse([]))

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.text()).toContain('Chưa có AI Proposal')
    expect(wrapper.text()).not.toContain('Generate')
    expect(wrapper.find('[name="audience_summary"]').exists()).toBe(false)
  })

  it('renders a retryable error state when proposal list request fails after project loads', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse({ error: { code: 'request_failed' } }, 503))

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.text()).toContain('Không thể kết nối máy chủ.')
    expect(wrapper.text()).not.toContain('Chưa có AI Proposal')
    expect(wrapper.find('[data-testid="retry-proposal-list"]').exists()).toBe(true)

    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))

    await wrapper.find('[data-testid="retry-proposal-list"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Tieu de A')
    expect(wrapper.text()).not.toContain('Không thể kết nối máy chủ.')
  })

  it('renders newest version history with status labels and opens the newest proposal', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary, approvedSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))

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
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))
      .mockResolvedValueOnce(jsonResponse({ ...draftProposal, revision: 5, audience_summary: 'Nguoi xem moi' }))

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
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))
      .mockResolvedValueOnce(jsonResponse({ ...draftProposal, revision: 5, audience_summary: 'Nguoi xem moi' }))

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
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: 'validation_failed',
              message: 'Request validation failed.',
              fields: { audience_summary: 'required' },
            },
          },
          400,
        ),
      )

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

  it('does not expose a load retry after a failed mutation following a failed version switch', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary, approvedSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))
      .mockResolvedValueOnce(jsonResponse({ error: { code: 'request_failed' } }, 503))
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: 'validation_failed',
              message: 'Request validation failed.',
              fields: { audience_summary: 'required' },
            },
          },
          400,
        ),
      )

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

  it('shows stale save conflict without auto-overwrite', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: 'STALE_REVISION',
              message: 'Creative proposal revision is stale.',
            },
          },
          409,
        ),
      )

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[name="audience_summary"]').setValue('Thay doi xung dot')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('Proposal trên máy chủ đã thay đổi')
    expect((wrapper.find('[name="audience_summary"]').element as HTMLTextAreaElement).value).toBe('Thay doi xung dot')
    expect(
      fetchMock.mock.calls.filter(
        ([url, init]) =>
          url === `/api/v1/projects/${projectId}/creative-proposals/2` && init?.method === 'PUT',
      ),
    ).toHaveLength(1)
  })

  it('preserves dirty edits when stale recovery fails to reload the latest version', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary, approvedSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: 'STALE_REVISION',
              message: 'Creative proposal revision is stale.',
            },
          },
          409,
        ),
      )
      .mockResolvedValueOnce(jsonResponse({ error: { code: 'request_failed' } }, 503))

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
    expect(fetchMock.mock.calls.filter(([url]) => url === `/api/v1/projects/${projectId}/creative-proposals/1`)).toHaveLength(0)
  })

  it('shows a visible retry state when the initial proposal version cannot be loaded', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary]))
      .mockResolvedValueOnce(jsonResponse({ error: { code: 'request_failed' } }, 503))

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.text()).toContain('Không thể kết nối máy chủ.')
    expect(wrapper.find('[data-testid="retry-proposal-load"]').exists()).toBe(true)
    expect(wrapper.find('[name="audience_summary"]').exists()).toBe(false)
  })

  it('renders approved and superseded versions as read-only', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([approvedSummary]))
      .mockResolvedValueOnce(jsonResponse(approvedProposal))

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    expect(wrapper.text()).toContain('Phiên bản này chỉ đọc')
    expect(wrapper.find('[name="audience_summary"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="approve-proposal"]').exists()).toBe(false)
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()
  })

  it('explicitly approves a draft with the current revision and renders approved state after success', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))
      .mockResolvedValueOnce(
        jsonResponse({
          ...draftProposal,
          status: 'approved',
          approved_at: '2026-08-31T10:00:00Z',
        }),
      )

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
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([draftSummary, approvedSummary]))
      .mockResolvedValueOnce(jsonResponse(draftProposal))
      .mockResolvedValueOnce(jsonResponse(approvedProposal))

    const wrapper = await mountCreativeProposalView()
    await flushPromises()

    await wrapper.find('[name="audience_summary"]').setValue('Dang sua')
    await wrapper.find('[data-testid="version-1"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Chọn Hủy thay đổi')
    expect((wrapper.find('[name="audience_summary"]').element as HTMLTextAreaElement).value).toBe('Dang sua')
    expect(fetchMock).not.toHaveBeenCalledWith(
      `/api/v1/projects/${projectId}/creative-proposals/1`,
      expect.anything(),
    )

    await wrapper.find('[data-testid="discard-version-switch"]').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      `/api/v1/projects/${projectId}/creative-proposals/1`,
      expect.anything(),
    )
    expect(wrapper.text()).toContain('Tieu de da duyet')
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

function flushPromises() {
  return new Promise((resolve) => window.setTimeout(resolve))
}
