import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
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

afterEach(() => {
  vi.unstubAllGlobals()
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

  it('does not turn a failed Scene Plan list into a false no-approved-plan state', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, { error: { code: 'request_failed' } }, 503)

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="scene-plan-list-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="scene-assignment-disabled"]').exists()).toBe(false)
  })

  it('retries the exact approved plan version after its initial load fails', async () => {
    let planAttempts = 0
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary])
    routeFn('GET', `/api/v1/projects/${projectId}/scene-plans/2`, () => {
      planAttempts += 1
      return planAttempts === 1 ? jsonResponse({ error: { code: 'request_failed' } }, 503) : jsonResponse(approvedPlan)
    })
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual' },
      { scene_key: 'scene-body-1', role: 'primary_visual' },
    ])

    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="scene-plan-error"] .secondary-button').exists()).toBe(true)
    expect(wrapper.find('[data-testid="media-asset-' + imageId + '"]').exists()).toBe(true)

    await wrapper.find('[data-testid="scene-plan-error"] .secondary-button').trigger('click')
    await flushPromises()

    expect(planAttempts).toBe(2)
    expect(wrapper.find('[data-testid="scene-row-scene-intro-1"]').exists()).toBe(true)
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

  it('uploads a supported file as multipart data and inserts the returned asset', async () => {
    const uploaded = { ...image, original_filename: 'uploaded.png' }
    vi.stubGlobal('XMLHttpRequest', undefined)
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [])
    route('POST', `/api/v1/projects/${projectId}/media-assets`, uploaded, 201)

    const wrapper = await mountView()
    await flushPromises()
    const input = wrapper.find('input[type="file"]')
    const supported = new File(['png'], 'uploaded.png', { type: 'image/png' })
    Object.defineProperty(input.element, 'files', { value: [supported], configurable: true })
    await input.trigger('change')
    await flushPromises()

    expect(wrapper.find(`[data-testid="media-asset-${imageId}"]`).exists()).toBe(true)
    const uploadCall = fetchMock.mock.calls.find(([url, init]) => url === `/api/v1/projects/${projectId}/media-assets` && init?.method === 'POST')
    expect(uploadCall?.[1]?.body).toBeInstanceOf(FormData)
    expect((uploadCall?.[1]?.headers as Record<string, string> | undefined)?.['Content-Type']).toBeUndefined()
  })

  it('supports cancelling an in-flight upload without changing the library', async () => {
    vi.stubGlobal('XMLHttpRequest', undefined)
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [])
    fetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (url === `/api/v1/projects/${projectId}/media-assets` && init?.method === 'POST') {
        return new Promise((_resolve, reject) => {
          init.signal?.addEventListener('abort', () => reject(new DOMException('cancelled', 'AbortError')))
        })
      }
      const method = init?.method ?? 'GET'
      const match = [...handlers].reverse().find((item) => (!item.method || item.method === method) && item.path === url)
      return Promise.resolve(match ? match.handler(url, init) : jsonResponse({ error: { code: 'not_found' } }, 404))
    })

    const wrapper = await mountView()
    await flushPromises()
    const input = wrapper.find('input[type="file"]')
    const supported = new File(['png'], 'pending.png', { type: 'image/png' })
    Object.defineProperty(input.element, 'files', { value: [supported], configurable: true })
    await input.trigger('change')
    await flushPromises()
    await wrapper.find('[data-testid="cancel-upload"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="upload-error"]').exists()).toBe(false)
    expect(wrapper.find(`[data-testid="media-asset-${imageId}"]`).exists()).toBe(true)
  })

  it('presents a transport failure without removing existing assets', async () => {
    vi.stubGlobal('XMLHttpRequest', undefined)
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [])
    route('POST', `/api/v1/projects/${projectId}/media-assets`, { error: { code: 'MEDIA_ASSET_STORAGE_FAILED' } }, 502)

    const wrapper = await mountView()
    await flushPromises()
    const input = wrapper.find('input[type="file"]')
    const supported = new File(['png'], 'failed.png', { type: 'image/png' })
    Object.defineProperty(input.element, 'files', { value: [supported], configurable: true })
    await input.trigger('change')
    await flushPromises()

    expect(wrapper.find('[data-testid="upload-error"]').exists()).toBe(true)
    expect(wrapper.find(`[data-testid="media-asset-${imageId}"]`).exists()).toBe(true)
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

  it('renders scene plan list error notice and does not show false no-approved-plan guidance', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, { error: { code: 'request_failed' } }, 500)

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="scene-plan-list-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="scene-assignment-disabled"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Chưa có Scene Plan đã duyệt')
  })

  it('allows retrying selected plan load after failure without losing loaded media', async () => {
    let callCount = 0
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary])
    handlers.push({
      method: 'GET',
      path: `/api/v1/projects/${projectId}/scene-plans/2`,
      handler: () => {
        callCount++
        if (callCount === 1) return jsonResponse({ error: { code: 'request_failed' } }, 500)
        return jsonResponse(approvedPlan, 200)
      },
    })
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual' },
      { scene_key: 'scene-body-1', role: 'primary_visual' },
    ])

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="scene-plan-error"]').exists()).toBe(true)
    expect(wrapper.find(`[data-testid="media-asset-${imageId}"]`).exists()).toBe(true)

    await wrapper.find('[data-testid="scene-plan-error"] .secondary-button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="scene-plan-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="scene-row-scene-intro-1"]').exists()).toBe(true)
  })

  it('scopes assignment errors and completion to exact plan version without corrupting newer plan', async () => {
    let rejectV2Assign!: (error: Error) => void
    const v2AssignPromise = new Promise((_, reject) => { rejectV2Assign = reject })

    const secondSummary = { ...approvedSummary, version: 3 }
    const secondPlan = { ...approvedPlan, version: 3, scenes: [{ ...approvedPlan.scenes[0], key: 'scene-intro-1' }] }

    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary, secondSummary])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/3`, secondPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual' },
    ])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/3/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual' },
    ])
    handlers.push({
      method: 'PUT',
      path: `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual`,
      handler: () => v2AssignPromise,
    })

    const wrapper = await mountView()
    await flushPromises()

    // Trigger assignment on v2
    await wrapper.find(`[data-testid="asset-option-${imageId}-scene-intro-1"]`).trigger('click')

    // Switch to v3 before v2 assignment finishes
    await wrapper.findAll('select')[2]?.setValue('3')
    await flushPromises()

    expect(wrapper.find('[data-testid="scene-row-scene-intro-1"]').exists()).toBe(true)

    // Now v2 assignment fails late
    rejectV2Assign(new Error('late failure'))
    await flushPromises()

    // Version 3 must not show the v2 assignment error
    expect(wrapper.find('.error-text').exists()).toBe(false)
  })

  it('scopes assignment completion and errors across project route switch', async () => {
    let rejectProj1Assign!: (error: Error) => void
    const proj1AssignPromise = new Promise((_, reject) => { rejectProj1Assign = reject })

    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual' },
    ])
    handlers.push({
      method: 'PUT',
      path: `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual`,
      handler: () => proj1AssignPromise,
    })

    route('GET', `/api/v1/projects/${otherProjectId}`, otherProject)
    route('GET', `/api/v1/projects/${otherProjectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${otherProjectId}/scene-plans`, [approvedSummary])
    route('GET', `/api/v1/projects/${otherProjectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${otherProjectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual' },
    ])

    const wrapper = await mountView(projectId)
    await flushPromises()

    // Start assign on Project 1
    await wrapper.find(`[data-testid="asset-option-${imageId}-scene-intro-1"]`).trigger('click')

    // Route switch to Project 2
    await wrapper.vm.$router.push(`/projects/${otherProjectId}/media`)
    await flushPromises()

    // Project 1 assignment fails late
    rejectProj1Assign(new Error('delayed error'))
    await flushPromises()

    expect(wrapper.text()).toContain('Du an khac')
    expect(wrapper.find('.error-text').exists()).toBe(false)
  })

  it('renders replacement history with binding version, status, time, provenance, and video/image preview', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image, video] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 2, asset_id: videoId, status: 'active', assigned_at: '2026-08-31T10:00:00Z' }, asset: video },
    ])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual/history`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 2, asset_id: videoId, status: 'active', assigned_at: '2026-08-31T10:00:00Z' }, asset: video },
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 1, asset_id: imageId, status: 'replaced', assigned_at: '2026-08-31T09:00:00Z' }, asset: image },
    ])

    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('[data-testid="history-toggle-scene-intro-1"]').trigger('click')
    await flushPromises()

    const historyContainer = wrapper.find('[data-testid="scene-history-scene-intro-1"]')
    expect(historyContainer.exists()).toBe(true)

    // Check video preview in history
    expect(historyContainer.find('video').exists()).toBe(true)
    // Check image preview in history
    expect(historyContainer.find('img').exists()).toBe(true)
    // Check binding version and status labels
    expect(historyContainer.text()).toContain('Bản gán 2')
    expect(historyContainer.text()).toContain('Đang áp dụng')
    expect(historyContainer.text()).toContain('Bản gán 1')
    expect(historyContainer.text()).toContain('Đã thay thế')
    expect(historyContainer.text()).toContain('intro.png')
    expect(historyContainer.text()).toContain('demo.mp4')
  })

  it('preserves the old current visual during replacement until server success', async () => {
    let resolveAssign!: (value: unknown) => void
    const assignPromise = new Promise((resolve) => { resolveAssign = resolve })

    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image, video] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 1, asset_id: imageId, status: 'active' }, asset: image },
    ])
    handlers.push({
      method: 'PUT',
      path: `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual`,
      handler: () => assignPromise,
    })

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find(`[data-testid="scene-current-${imageId}"]`).exists()).toBe(true)

    // Click assign video
    await wrapper.find(`[data-testid="asset-option-${videoId}-scene-intro-1"]`).trigger('click')

    // While in-flight, image visual is preserved
    expect(wrapper.find(`[data-testid="scene-current-${imageId}"]`).exists()).toBe(true)
    expect(wrapper.find(`[data-testid="scene-current-${videoId}"]`).exists()).toBe(false)
    expect(wrapper.text()).toContain('Đang lưu gán visual...')

    // Now server succeeds
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 2, asset_id: videoId, status: 'active' }, asset: video },
    ])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual/history`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 2, asset_id: videoId }, asset: video },
    ])
    resolveAssign(jsonResponse({
      scene_key: 'scene-intro-1',
      role: 'primary_visual',
      binding: { binding_version: 2, asset_id: videoId, status: 'active' },
      asset: video,
    }))
    await flushPromises()

    expect(wrapper.find(`[data-testid="scene-current-${videoId}"]`).exists()).toBe(true)
  })

  it('supports idempotent same-asset assignment', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 1, asset_id: imageId, status: 'active' }, asset: image },
    ])
    route('PUT', `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual`, {
      scene_key: 'scene-intro-1',
      role: 'primary_visual',
      binding: { binding_version: 2, asset_id: imageId, status: 'active' },
      asset: image,
    })
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual/history`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 2, asset_id: imageId }, asset: image },
    ])

    const wrapper = await mountView()
    await flushPromises()

    // Assign same asset again
    await wrapper.find(`[data-testid="asset-option-${imageId}-scene-intro-1"]`).trigger('click')
    await flushPromises()

    expect(wrapper.find(`[data-testid="scene-current-${imageId}"]`).exists()).toBe(true)
    expect(wrapper.find('.error-text').exists()).toBe(false)
  })

  it('restores a historical asset by triggering a normal new assignment', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/media-assets`, { assets: [image, video] })
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedSummary])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 2, asset_id: videoId, status: 'active' }, asset: video },
    ])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual/history`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 2, asset_id: videoId, status: 'active' }, asset: video },
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 1, asset_id: imageId, status: 'replaced' }, asset: image },
    ])
    route('PUT', `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual`, {
      scene_key: 'scene-intro-1',
      role: 'primary_visual',
      binding: { binding_version: 3, asset_id: imageId, status: 'active' },
      asset: image,
    })

    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('[data-testid="history-toggle-scene-intro-1"]').trigger('click')
    await flushPromises()

    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/media-bindings`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 3, asset_id: imageId, status: 'active' }, asset: image },
    ])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual/history`, [
      { scene_key: 'scene-intro-1', role: 'primary_visual', binding: { binding_version: 3, asset_id: imageId, status: 'active' }, asset: image },
    ])

    // Click restore on version 1 entry (image)
    await wrapper.find('[data-testid="restore-history-1"]').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      `/api/v1/projects/${projectId}/scene-plans/2/scenes/scene-intro-1/primary-visual`,
      expect.objectContaining({ method: 'PUT', body: JSON.stringify({ asset_id: imageId }) }),
    )
    expect(wrapper.find(`[data-testid="scene-current-${imageId}"]`).exists()).toBe(true)
  })
})

function route(method: string | undefined, path: string, body: unknown, status = 200) {
  handlers.push({ method, path, handler: () => jsonResponse(body, status) })
}

function routeFn(method: string | undefined, path: string, handler: Handler) {
  handlers.push({ method, path, handler })
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
