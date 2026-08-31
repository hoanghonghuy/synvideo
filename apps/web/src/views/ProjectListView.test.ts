import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
import ProjectCreateView from './ProjectCreateView.vue'
import ProjectListView from './ProjectListView.vue'

const fetchMock = vi.fn()

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
})

describe('ProjectListView', () => {
  it('renders an empty state when no projects exist', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ projects: [] }))

    const wrapper = mount(ProjectListView, {
      global: {
        plugins: [i18n, testRouter()],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Chưa có dự án nào được tạo.')
    expect(wrapper.text()).toContain('Tạo dự án')
  })

  it('renders persisted projects returned by the API', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        projects: [
          {
            id: '11111111-1111-4111-8111-111111111111',
            title: 'Video ra mat',
            description: '',
            content_format: 'short',
            aspect_ratio: '9:16',
            target_duration_seconds: 60,
            locale: 'vi',
            status: 'active',
            created_at: '2026-08-31T08:00:00Z',
            updated_at: '2026-08-31T08:00:00Z',
          },
        ],
      }),
    )

    const wrapper = mount(ProjectListView, {
      global: {
        plugins: [i18n, testRouter()],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Video ra mat')
    expect(wrapper.text()).toContain('Đang hoạt động')
  })
})

describe('ProjectCreateView', () => {
  it('submits project metadata and navigates to the created project', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        id: '33333333-3333-4333-8333-333333333333',
        title: 'Video moi',
        description: '',
        content_format: 'short',
        aspect_ratio: '9:16',
        target_duration_seconds: null,
        locale: 'vi',
        status: 'active',
        created_at: '2026-08-31T08:00:00Z',
        updated_at: '2026-08-31T08:00:00Z',
      }),
    )
    const router = testRouter()
    router.push('/projects/new')
    await router.isReady()

    const wrapper = mount(ProjectCreateView, {
      global: {
        plugins: [i18n, router],
      },
    })

    await wrapper.find('input[name="title"]').setValue('Video moi')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects',
      expect.objectContaining({
        method: 'POST',
        body: expect.stringContaining('Video moi'),
      }),
    )
    expect(router.currentRoute.value.path).toBe('/projects/33333333-3333-4333-8333-333333333333')
  })

  it('renders API validation feedback for create flow', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        {
          error: {
            code: 'validation_failed',
            message: 'Request validation failed.',
            fields: { title: 'required' },
          },
        },
        400,
      ),
    )

    const wrapper = mount(ProjectCreateView, {
      global: {
        plugins: [i18n, testRouter()],
      },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.text()).toContain('Vui lòng kiểm tra lại thông tin dự án.')
    expect(wrapper.text()).toContain('Bắt buộc nhập.')
  })
})

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  }
}

function testRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: ProjectListView },
      { path: '/projects', component: ProjectListView },
      { path: '/projects/new', component: ProjectCreateView },
      { path: '/projects/:id', component: ProjectListView },
    ],
  })
}

function flushPromises() {
  return new Promise((resolve) => window.setTimeout(resolve))
}
