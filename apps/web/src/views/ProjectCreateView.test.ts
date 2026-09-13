import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
import ProjectCreateView from './ProjectCreateView.vue'

const fetchMock = vi.fn()

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
})

describe('ProjectCreateView mutation recovery', () => {
  it('focuses the request-level error after a generic create failure', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: { code: 'request_failed', message: 'failed' } }, 500))
    const wrapper = await mountCreateView()

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    const alert = wrapper.get('[role="alert"]')
    expect(alert.attributes('tabindex')).toBe('-1')
    expect(document.activeElement).toBe(alert.element)
    wrapper.unmount()
  })

  it('preserves field-level focus when create validation fails', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: { code: 'validation_failed', message: 'invalid', fields: { title: 'required' } } }, 422))
    const wrapper = await mountCreateView()

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(document.activeElement).toBe(wrapper.get('input[name="title"]').element)
    wrapper.unmount()
  })
})

async function mountCreateView() {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/projects', component: { template: '<div />' } },
    { path: '/projects/new', component: ProjectCreateView },
    { path: '/projects/:id', component: { template: '<div />' } },
  ] })
  router.push('/projects/new')
  await router.isReady()
  return mount(ProjectCreateView, { attachTo: document.body, global: { plugins: [i18n, router] } })
}

function jsonResponse(body: unknown, status = 200) {
  return { ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) }
}

function flushPromises() {
  return new Promise((resolve) => window.setTimeout(resolve))
}
