import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
import ScriptView from './ScriptView.vue'

const fetchMock = vi.fn()
const projectId = '11111111-1111-4111-8111-111111111111'
const jobId = '22222222-2222-4222-8222-222222222222'

const project = {
  id: projectId,
  title: 'Video kịch bản',
  description: 'Mô tả dự án',
  content_format: 'long',
  aspect_ratio: '16:9',
  target_duration_seconds: 600,
  locale: 'vi',
  status: 'active',
  created_at: '2026-08-31T08:00:00Z',
  updated_at: '2026-08-31T08:30:00Z',
}

const approvedProposal = {
  version: 2,
  revision: 1,
  status: 'approved',
  source_brief_revision: 1,
  created_at: '2026-08-31T08:00:00Z',
  updated_at: '2026-08-31T08:30:00Z',
  approved_at: '2026-08-31T08:31:00Z',
}

const draftScript = {
  project_id: projectId,
  version: 1,
  revision: 2,
  status: 'draft',
  source_proposal_version: 2,
  content_locale: 'vi',
  sections: [{ key: 'intro', heading: 'Mở đầu', body: 'Nội dung mở đầu' }],
  estimated_duration_seconds: 120,
  notes: 'Ghi chú',
  created_at: '2026-08-31T09:00:00Z',
  updated_at: '2026-08-31T09:30:00Z',
  approved_at: null,
}

const approvedScript = {
  ...draftScript,
  version: 2,
  revision: 1,
  status: 'approved',
  approved_at: '2026-08-31T09:31:00Z',
}

const providers = { providers: [{ id: 'provider-a', display_name: 'Provider A', models: [{ id: 'model-a', display_name: 'Model A' }] }] }

type Handler = (url: string, init?: RequestInit) => unknown
let handlers: { method?: string; path: string; handler: Handler }[] = []

beforeEach(() => {
  handlers = []
  window.sessionStorage.clear()
  fetchMock.mockReset()
  fetchMock.mockImplementation((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    const match = [...handlers].reverse().find((item) => {
      return (!item.method || item.method === method) && item.path === url
    })
    if (match) return Promise.resolve(match.handler(url, init))
    if (url === '/api/v1/ai/text-generation-options') return Promise.resolve(jsonResponse({ providers: [] }))
    return Promise.resolve(jsonResponse({ error: { code: 'not_found' } }, 404))
  })
  vi.stubGlobal('fetch', fetchMock)
  i18n.global.locale.value = 'vi'
})

describe('ScriptView', () => {
  it('renders an empty readiness state when there is no script and no approved proposal', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [])
    route('GET', `/api/v1/projects/${projectId}/creative-proposals`, [])
    route('GET', '/api/v1/ai/text-generation-options', providers)

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="script-empty-state"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="generate-script-btn"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('Cần duyệt AI Proposal trước khi tạo kịch bản.')
  })

  it('preserves dirty edits and blocks history switching and generation', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript, draftScript])
    route('GET', `/api/v1/projects/${projectId}/scripts/1`, draftScript)
    route('GET', `/api/v1/projects/${projectId}/scripts/2`, approvedScript)
    route('GET', `/api/v1/projects/${projectId}/creative-proposals`, [approvedProposal])
    route('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, approvedProposal)
    route('GET', '/api/v1/ai/text-generation-options', providers)

    const wrapper = await mountView()
    await flushPromises()
    await wrapper.find('[name="section_body_0"]').setValue('Nội dung đã chỉnh sửa')
    expect(wrapper.find('[data-testid="dirty-state"]').exists()).toBe(true)

    expect(wrapper.find('[data-testid="generate-script-btn"]').attributes('disabled')).toBeDefined()
    await wrapper.find('[data-testid="version-2"]').trigger('click')
    expect(wrapper.find('[data-testid="dirty-switch-warning"]').exists()).toBe(true)
    expect(fetchMock.mock.calls.filter(([url, init]) => url.endsWith('/scripts/2') && (!init || !init.method)).length).toBe(0)
  })

  it('shows source staleness and preserves local edits after a stale save response', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [draftScript])
    route('GET', `/api/v1/projects/${projectId}/scripts/1`, draftScript)
    route('GET', `/api/v1/projects/${projectId}/creative-proposals`, [{ ...approvedProposal, version: 3 }])
    route('GET', '/api/v1/ai/text-generation-options', providers)
    route('PUT', `/api/v1/projects/${projectId}/scripts/1`, { error: { code: 'STALE_REVISION' } }, 409)

    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="stale-source-warning"]').exists()).toBe(true)
    await wrapper.find('[name="section_body_0"]').setValue('Bản chỉnh sửa cần được giữ lại')
    await wrapper.find('form.script-form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="dirty-state"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="script-load-error"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Script trên máy chủ đã thay đổi.')
    expect((wrapper.find('[name="section_body_0"]').element as HTMLTextAreaElement).value).toBe('Bản chỉnh sửa cần được giữ lại')
  })

  it('renders a retryable script load error instead of a false empty state', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scripts`, { error: { code: 'request_failed' } }, 503)
    route('GET', `/api/v1/projects/${projectId}/creative-proposals`, [])

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="script-empty-state"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="retry-script-list"]').exists()).toBe(true)
  })

  it('polls the same job and loads exactly the succeeded script version', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      route('GET', `/api/v1/projects/${projectId}`, project)
      let listCalls = 0
      routeFn('GET', `/api/v1/projects/${projectId}/scripts`, () => {
        listCalls++
        return jsonResponse(listCalls === 1 ? [] : [{ ...draftScript, version: 7 }])
      })
      route('GET', `/api/v1/projects/${projectId}/creative-proposals`, [approvedProposal])
      route('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, approvedProposal)
      route('GET', '/api/v1/ai/text-generation-options', providers)
      route('POST', `/api/v1/projects/${projectId}/script-generations`, job('queued'), 202)
      route('GET', `/api/v1/projects/${projectId}/script-generations/${jobId}`, job('succeeded', 7))
      route('GET', `/api/v1/projects/${projectId}/scripts/7`, { ...draftScript, version: 7 })

      const wrapper = await mountView()
      await flushPromises()
      await wrapper.find('[data-testid="generate-script-btn"]').trigger('click')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      await flushPromises()

      expect(wrapper.text()).toContain('Phiên bản 7')
      expect(fetchMock.mock.calls.filter(([url, init]) => url === `/api/v1/projects/${projectId}/script-generations` && init?.method === 'POST')).toHaveLength(1)
      expect(fetchMock.mock.calls.some(([url]) => url === `/api/v1/projects/${projectId}/scripts/7`)).toBe(true)
    } finally {
      vi.useRealTimers()
    }
  })

  it('recovers a transient poll failure without posting a replacement job', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      window.sessionStorage.setItem(`script_job_${projectId}`, jobId)
      route('GET', `/api/v1/projects/${projectId}`, project)
      route('GET', `/api/v1/projects/${projectId}/scripts`, [draftScript])
      route('GET', `/api/v1/projects/${projectId}/scripts/1`, draftScript)
      route('GET', `/api/v1/projects/${projectId}/creative-proposals`, [approvedProposal])
      route('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, approvedProposal)
      route('GET', '/api/v1/ai/text-generation-options', providers)
      let polls = 0
      routeFn('GET', `/api/v1/projects/${projectId}/script-generations/${jobId}`, () => {
        polls++
        return polls === 1 ? jsonResponse({ error: { code: 'request_failed' } }, 503) : jsonResponse(job('running'))
      })

      const wrapper = await mountView()
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      expect(wrapper.find('[data-testid="job-error-banner"]').exists()).toBe(true)
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      expect(polls).toBe(2)
      expect(fetchMock.mock.calls.filter(([url, init]) => url === `/api/v1/projects/${projectId}/script-generations` && init?.method === 'POST')).toHaveLength(0)
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps a succeeded job for exact-version load retry without regenerating', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      route('GET', `/api/v1/projects/${projectId}`, project)
      let listCalls = 0
      routeFn('GET', `/api/v1/projects/${projectId}/scripts`, () => {
        listCalls++
        return jsonResponse(listCalls === 1 ? [] : [{ ...draftScript, version: 7 }])
      })
      route('GET', `/api/v1/projects/${projectId}/creative-proposals`, [approvedProposal])
      route('GET', `/api/v1/projects/${projectId}/creative-proposals/2`, approvedProposal)
      route('GET', '/api/v1/ai/text-generation-options', providers)
      route('POST', `/api/v1/projects/${projectId}/script-generations`, job('queued'), 202)
      route('GET', `/api/v1/projects/${projectId}/script-generations/${jobId}`, job('succeeded', 7))
      let loads = 0
      routeFn('GET', `/api/v1/projects/${projectId}/scripts/7`, () => {
        loads++
        return loads === 1 ? jsonResponse({ error: { code: 'request_failed' } }, 503) : jsonResponse({ ...draftScript, version: 7 })
      })

      const wrapper = await mountView()
      await flushPromises()
      await wrapper.find('[data-testid="generate-script-btn"]').trigger('click')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      await flushPromises()

      expect(wrapper.find('[data-testid="job-succeeded-load-failed-banner"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="generate-script-btn"]').attributes('disabled')).toBeDefined()
      await wrapper.find('[data-testid="retry-load-generated-btn"]').trigger('click')
      await flushPromises()
      expect(wrapper.text()).toContain('Phiên bản 7')
      expect(fetchMock.mock.calls.filter(([url, init]) => url === `/api/v1/projects/${projectId}/script-generations` && init?.method === 'POST')).toHaveLength(1)
    } finally {
      vi.useRealTimers()
    }
  })
})

async function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/projects/:id/script', component: ScriptView },
      { path: '/projects/:id', component: { template: '<div />' } },
      { path: '/projects/:id/creative-proposal', component: { template: '<div />' } },
      { path: '/settings/ai-providers', component: { template: '<div />' } },
    ],
  })
  await router.push(`/projects/${projectId}/script`)
  await router.isReady()
  return mount(ScriptView, { global: { plugins: [router, i18n] } })
}

function route(method: string, path: string, body: unknown, status = 200) {
  routeFn(method, path, () => jsonResponse(body, status))
}

function routeFn(method: string, path: string, handler: Handler) {
  handlers.push({ method, path, handler })
}

function job(state: 'queued' | 'running' | 'succeeded' | 'failed', scriptVersion: number | null = null) {
  return {
    id: jobId,
    state,
    attempt: state === 'queued' ? 0 : 1,
    max_attempts: 3,
    error_code: null,
    script_version: scriptVersion,
    created_at: '2026-08-31T09:00:00Z',
    updated_at: '2026-08-31T09:00:00Z',
  }
}

function jsonResponse(body: unknown, status = 200) {
  return { ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) }
}
