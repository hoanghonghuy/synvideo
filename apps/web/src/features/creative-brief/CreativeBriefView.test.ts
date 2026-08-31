import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
import CreativeBriefView from './CreativeBriefView.vue'

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

const existingBrief = {
  project_id: projectId,
  revision: 2,
  source_text: 'Noi dung goc',
  target_audience: 'Gen Z',
  objective: 'Tang nhan dien',
  desired_style: 'Nang dong',
  tone: 'Than thien',
  distribution_targets: ['youtube', 'tiktok'],
  call_to_action: 'Theo doi kenh',
  must_include: ['Logo thuong hieu'],
  must_avoid: ['Am thanh to'],
  created_at: '2026-08-31T08:00:00Z',
  updated_at: '2026-08-31T09:00:00Z',
}

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
  i18n.global.locale.value = 'vi'
})

describe('CreativeBriefView', () => {
  it('renders a new draft when GET returns creative_brief_not_found', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(
        jsonResponse(
          { error: { code: 'creative_brief_not_found', message: 'Creative brief was not found.' } },
          404,
        ),
      )

    const wrapper = await mountCreativeBriefView()
    await flushPromises()

    expect(wrapper.text()).toContain('Bản nháp mới')
    expect(wrapper.find('[name="source_text"]').exists()).toBe(true)
    expect((wrapper.find('[name="source_text"]').element as HTMLTextAreaElement).value).toBe('')
  })

  it('sends PUT create payload without revision for a new brief', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(
        jsonResponse(
          { error: { code: 'creative_brief_not_found', message: 'Creative brief was not found.' } },
          404,
        ),
      )
      .mockResolvedValueOnce(
        jsonResponse({ ...existingBrief, revision: 1, source_text: 'Y tuong moi' }, 201),
      )

    const wrapper = await mountCreativeBriefView()
    await flushPromises()

    await wrapper.find('[name="source_text"]').setValue('Y tuong moi')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    const putCall = fetchMock.mock.calls.find(
      ([url, init]) =>
        url === `/api/v1/projects/${projectId}/creative-brief` && init?.method === 'PUT',
    )
    expect(putCall).toBeDefined()
    expect(JSON.parse(String(putCall?.[1]?.body))).toEqual({
      source_text: 'Y tuong moi',
      target_audience: '',
      objective: '',
      desired_style: '',
      tone: '',
      distribution_targets: [],
      call_to_action: '',
      must_include: [],
      must_avoid: [],
    })
    expect(wrapper.text()).toContain('Đã lưu')
  })

  it('clears saved indicator after the user edits again', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(
        jsonResponse(
          { error: { code: 'creative_brief_not_found', message: 'Creative brief was not found.' } },
          404,
        ),
      )
      .mockResolvedValueOnce(
        jsonResponse({ ...existingBrief, revision: 1, source_text: 'Y tuong moi' }, 201),
      )

    const wrapper = await mountCreativeBriefView()
    await flushPromises()

    await wrapper.find('[name="source_text"]').setValue('Y tuong moi')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('Đã lưu Creative Brief.')

    await wrapper.find('[name="source_text"]').setValue('Chinh sua sau khi luu')
    await flushPromises()

    expect(wrapper.text()).not.toContain('Đã lưu Creative Brief.')
    expect(wrapper.text()).toContain('Có thay đổi chưa lưu')
  })

  it('sends current revision on update and stores the incremented revision', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse(existingBrief))
      .mockResolvedValueOnce(
        jsonResponse({ ...existingBrief, revision: 3, source_text: 'Cap nhat' }),
      )

    const wrapper = await mountCreativeBriefView()
    await flushPromises()

    await wrapper.find('[name="source_text"]').setValue('Cap nhat')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    const putCall = fetchMock.mock.calls.find(
      ([url, init]) =>
        url === `/api/v1/projects/${projectId}/creative-brief` && init?.method === 'PUT',
    )
    expect(JSON.parse(String(putCall?.[1]?.body)).revision).toBe(2)

    await wrapper.find('[name="source_text"]').setValue('Lan hai')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    const secondPut = fetchMock.mock.calls.filter(
      ([url, init]) =>
        url === `/api/v1/projects/${projectId}/creative-brief` && init?.method === 'PUT',
    )[1]
    expect(JSON.parse(String(secondPut?.[1]?.body)).revision).toBe(3)
  })

  it('preserves entered values after validation failure', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(
        jsonResponse(
          { error: { code: 'creative_brief_not_found', message: 'Creative brief was not found.' } },
          404,
        ),
      )
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: 'validation_failed',
              message: 'Request validation failed.',
              fields: { source_text: 'required' },
            },
          },
          400,
        ),
      )

    const wrapper = await mountCreativeBriefView()
    await flushPromises()

    await wrapper.find('[name="source_text"]').setValue('   ')
    await wrapper.find('[name="objective"]').setValue('Muc tieu giu lai')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect((wrapper.find('[name="source_text"]').element as HTMLTextAreaElement).value).toBe('   ')
    expect((wrapper.find('[name="objective"]').element as HTMLInputElement).value).toBe('Muc tieu giu lai')
    expect(wrapper.text()).toContain('Vui lòng kiểm tra lại')
  })

  it('preserves entered values after network failure', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(
        jsonResponse(
          { error: { code: 'creative_brief_not_found', message: 'Creative brief was not found.' } },
          404,
        ),
      )
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))

    const wrapper = await mountCreativeBriefView()
    await flushPromises()

    await wrapper.find('[name="source_text"]').setValue('Van con o day')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect((wrapper.find('[name="source_text"]').element as HTMLTextAreaElement).value).toBe('Van con o day')
    expect(wrapper.text()).toContain('Không thể kết nối máy chủ')
  })

  it('shows stale conflict state and does not auto-retry overwrite', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse(existingBrief))
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: 'STALE_REVISION',
              message: 'Creative brief revision is stale.',
            },
          },
          409,
        ),
      )

    const wrapper = await mountCreativeBriefView()
    await flushPromises()

    await wrapper.find('[name="source_text"]').setValue('Thay doi xung dot')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('Phiên bản trên máy chủ đã thay đổi')
    expect((wrapper.find('[name="source_text"]').element as HTMLTextAreaElement).value).toBe('Thay doi xung dot')

    const putCalls = fetchMock.mock.calls.filter(
      ([url, init]) =>
        url === `/api/v1/projects/${projectId}/creative-brief` && init?.method === 'PUT',
    )
    expect(putCalls).toHaveLength(1)
  })

  it('reloads latest server brief after explicit reload action', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse(existingBrief))
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: 'STALE_REVISION',
              message: 'Creative brief revision is stale.',
            },
          },
          409,
        ),
      )
      .mockResolvedValueOnce(
        jsonResponse({ ...existingBrief, revision: 4, source_text: 'Phien ban moi nhat' }),
      )

    const wrapper = await mountCreativeBriefView()
    await flushPromises()

    await wrapper.find('[name="source_text"]').setValue('Thay doi xung dot')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    await wrapper.find('[data-testid="reload-latest"]').trigger('click')
    await flushPromises()

    expect((wrapper.find('[name="source_text"]').element as HTMLTextAreaElement).value).toBe('Phien ban moi nhat')
    expect(wrapper.text()).not.toContain('Phiên bản trên máy chủ đã thay đổi')
  })
})

async function mountCreativeBriefView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/projects/:id', component: { template: '<div />' } },
      { path: '/projects/:id/creative-brief', component: CreativeBriefView },
    ],
  })
  router.push(`/projects/${projectId}/creative-brief`)
  await router.isReady()

  return mount(CreativeBriefView, {
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
