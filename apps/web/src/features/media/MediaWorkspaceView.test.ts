import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
import MediaWorkspaceView from './MediaWorkspaceView.vue'

const fetchMock = vi.fn()
const projectId = '11111111-1111-4111-8111-111111111111'
const otherProjectId = '22222222-2222-4222-8222-222222222222'
const imageId = '33333333-3333-4333-8333-333333333333'
const videoId = '44444444-4444-4444-8444-444444444444'

const project = {
  id: projectId,
  title: 'Video phan canh',
  description: 'Mo ta du an',
  content_format: 'short',
  aspect_ratio: '16:9',
  target_duration_seconds: 60,
  locale: 'vi',
  status: 'active',
  created_at: '2026-08-31T08:00:00Z',
  updated_at: '2026-08-31T08:30:00Z',
}

const otherProject = { ...project, id: otherProjectId, title: 'Du an khac' }
const image = {
  id: imageId,
  project_id: projectId,
  kind: 'image',
  origin: 'upload',
  mime_type: 'image/png',
  byte_size: 2048,
  sha256: 'a'.repeat(64),
  original_filename: 'intro.png',
  metadata: { width: 1280, height: 720 },
  created_at: '2026-08-31T09:00:00Z',
  updated_at: '2026-08-31T09:00:00Z',
}
const video = {
  ...image,
  id: videoId,
  kind: 'video',
  mime_type: 'video/mp4',
  original_filename: 'demo.mp4',
}

const approvedSummary = {
  version: 2,
  revision: 1,
  status: 'approved',
  source_script_version: 1,
  source_proposal_version: 1,
  content_locale: 'vi',
  created_at: '2026-08-31T09:00:00Z',
  updated_at: '2026-08-31T09:00:00Z',
  approved_at: '2026-08-31T09:05:00Z',
}

const approvedPlan = {
  ...approvedSummary,
  project_id: projectId,
  scenes: [
    {
      key: 'scene-intro-1',
      script_section_key: 'intro',
      narration: 'Chào mừng các bạn.',
      visual_instruction: 'Toàn cảnh trường quay.',
      planned_source_type: 'stock',
      expected_duration_seconds: 10,
    },
    {
      key: 'scene-body-1',
      script_section_key: 'body',
      narration: 'Nội dung chính của video.',
      visual_instruction: 'Cận cảnh giao diện.',
      planned_source_type: 'creator_media',
      expected_duration_seconds: 20,
    },
  ],
}

type Handler = (url: string, init?: RequestInit) => unknown
let handlers: { method?: string; path: string; handler: Handler }[] = []

beforeEach(() => {
  handlers = []
  fetchMock.mockReset()
  fetchMock.mockImplementation((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    const match = [...handlers].reverse().find((item) => {
      return (!item.method || item.method === method) && item.path === url
    })
    if (match) return Promise.resolve(match.handler(url, init))
    return Promise.resolve(jsonResponse({ error: { code: 'not_found' } }, 404))
  })
  vi.stubGlobal('fetch', fetchMock)
  vi.stubGlobal('confirm', vi.fn(() => true))
  i18n.global.locale.value = 'vi'
})

describe('MediaWorkspaceView', () => {
  it('keeps the library usable when there is no approved Scene Plan', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [])

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find(`[data-testid="media-asset-${imageId}"]`).exists()).toBe(true)
    expect(wrapper.find('[data-testid="scene-assignment-disabled"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Chưa có Scene Plan đã duyệt')
  })

  it('shows a media load error instead of a false empty library', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { error: { code: 'request_failed' } }, 503)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary])

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="media-library-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="media-empty-state"]').exists()).toBe(false)
  })

  it('isolates one preview failure from the rest of the library', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image, video] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [])

    const wrapper = await mountView()
    await flushPromises()
    const imagePreview = wrapper.find(`[data-testid="media-asset-${imageId}"] img`)
    await imagePreview.trigger('error')

    expect(wrapper.find(`[data-testid="media-asset-${imageId}"] [data-testid="preview-failure"]`).exists()).toBe(true)
    expect(wrapper.find(`[data-testid="media-asset-${videoId}"]`).exists()).toBe(true)
  })

  it('rejects unsupported uploads before transport and keeps the library stable', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [])

    const wrapper = await mountView()
    await flushPromises()
    const input = wrapper.find('input[type="file"]')
    const unsupported = new File(['text'], 'notes.txt', { type: 'text/plain' })
    Object.defineProperty(input.element, 'files', { value: [unsupported], configurable: true })
    await input.trigger('change')
    await flushPromises()

    expect(wrapper.find('[data-testid="upload-error"]').exists()).toBe(true)
    expect(wrapper.find(`[data-testid="media-asset-${imageId}"]`).exists()).toBe(true)
    expect(fetchMock).not.toHaveBeenCalledWith(
      `/api/v1/projects/${projectId}/media-assets`,
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('renders approved scenes in order and assigns a visual without optimistic replacement', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image, video] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual' },
      { scene_key: 'scene-body-1', role: 'primary_visual' },
    ])
    route('PUT', `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual`, {
      scene_key: 'scene-intro-1',
      role: 'primary_visual',
      binding: { binding_version: 1, asset_id: imageId, status: 'active' },
      asset: image,
    })
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual/history`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 1, asset_id: imageId }, asset: image },
    ])

    const wrapper = await mountView()
    await flushPromises()

    const scenes = wrapper.findAll('[data-testid^="scene-row-"]')
    expect(scenes).toHaveLength(2)
    expect(scenes[0]?.attributes('data-testid')).toBe('scene-row-scene-intro-1')
    expect(wrapper.text()).toContain('Chưa gán visual')

    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 1, asset_id: imageId }, asset: image },
      { scene_key: 'scene-body-1', role: 'primary_visual' },
    ])
    await wrapper.find(`[data-testid="asset-option-${imageId}-scene-intro-1"]`).trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual`,
      expect.objectContaining({ method: 'PUT', body: JSON.stringify({ asset_id: imageId }) }),
    )
    expect(wrapper.find(`[data-testid="scene-current-${imageId}"]`).exists()).toBe(true)
    expect(wrapper.find('[data-testid="scene-history-scene-intro-1"]').exists()).toBe(true)
  })

  it('keeps an in-use asset after delete conflict', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [])
    route('DELETE', `/api/v1/projects/${projectId}/media-assets/${imageId}`, { error: { code: 'MEDIA_ASSET_IN_USE' } }, 409)

    const wrapper = await mountView()
    await flushPromises()
    await wrapper.find(`[data-testid="delete-asset-${imageId}"]`).trigger('click')
    await wrapper.find('[data-testid="delete-confirmation"] .primary-button').trigger('click')
    await flushPromises()

    expect(wrapper.find(`[data-testid="media-asset-${imageId}"]`).exists()).toBe(true)
    expect(wrapper.find('[data-testid="delete-in-use-error"]').exists()).toBe(true)
  })

  it('loads exact bindings when switching approved plan versions', async () => {
    const secondSummary = { ...approvedSummary, version: 3 }
    const secondPlan = { ...approvedPlan, version: 3, scenes: [{ ...approvedPlan.scenes[0], key: 'scene-new-1' }] }
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary, secondSummary])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/3`, secondPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { asset_id: imageId }, asset: image },
      { scene_key: 'scene-body-1', role: 'primary_visual' },
    ])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/3/media-bindings`, [
      { scene_key: 'scene-new-1', role: 'primary_visual' },
    ])

    const wrapper = await mountView()
    await flushPromises()
    await wrapper.findAll('select')[2]?.setValue('3')
    await flushPromises()

    expect(wrapper.find('[data-testid="scene-row-scene-new-1"]').exists()).toBe(true)
    expect(wrapper.find(`[data-testid="scene-current-${imageId}"]`).exists()).toBe(false)
  })

  it('does not let a stale project response overwrite the current route', async () => {
    let resolveFirstProject!: (value: Response) => void
    const firstProjectResponse = new Promise<Response>((resolve) => { resolveFirstProject = resolve })
    fetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (url === `/api/v1/projects/${projectId}`) return firstProjectResponse
      if (url === `/api/v1/projects/${otherProjectId}`) return Promise.resolve(jsonResponse(otherProject))
      const method = init?.method ?? 'GET'
      const match = [...handlers].reverse().find((item) => (!item.method || item.method === method) && item.path === url)
      return Promise.resolve(match ? match.handler(url, init) : jsonResponse({ assets: [] }))
    })

    route('GET', `/api/v1/projects/${otherProjectId}/media-assets`, { assets: [] })
    route('GET', `/api/v1/projects/${otherProjectId}/scene-plans`, [])
    const wrapper = await mountView(projectId)
    await wrapper.vm.$router.push(`/projects/${otherProjectId}/media`)
    await flushPromises()
    resolveFirstProject(jsonResponse(project))
    await flushPromises()

    expect(wrapper.text()).toContain('Du an khac')
    expect(wrapper.text()).not.toContain('Video phan canh')
  })
})

function route(method: string | undefined, path: string, body: unknown, status = 200) {
  handlers.push({ method, path, handler: () => jsonResponse(body, status) })
}

async function mountView(initialProjectId = projectId) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/projects/:id/media', component: MediaWorkspaceView },
      { path: '/projects/:id', component: { template: '<div />' } },
    ],
  })
  await router.push(`/projects/${initialProjectId}/media`)
  await router.isReady()
  return mount(MediaWorkspaceView, { global: { plugins: [router, i18n] } })
}

function jsonResponse(body: unknown, status = 200) {
  return { ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) } as Response
}

function flushPromises() {
  return new Promise((resolve) => window.setTimeout(resolve))
}
