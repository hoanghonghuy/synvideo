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
  it('keeps submit focus and suppresses duplicate requests while create is pending', async () => {
    let resolveRequest!: (response: ReturnType<typeof jsonResponse>) => void
    fetchMock.mockReturnValueOnce(new Promise((resolve) => { resolveRequest = resolve }))
    const wrapper = await mountCreateView()
    const form = wrapper.get('form')
    const submit = wrapper.get('button[type="submit"]')
    ;(submit.element as HTMLButtonElement).focus()

    await form.trigger('submit')
    await wrapper.vm.$nextTick()

    const pendingForm = wrapper.get('form')
    const pendingSubmit = wrapper.get('button[type="submit"]')
    expect(pendingForm.attributes('aria-busy')).toBe('true')
    expect(pendingSubmit.attributes('aria-disabled')).toBe('true')
    expect(pendingSubmit.attributes('disabled')).toBeUndefined()
    expect(pendingSubmit.get('[role="status"]').attributes('aria-live')).toBe('polite')
    expect(pendingSubmit.get('[role="status"]').attributes('aria-atomic')).toBe('true')
    expect(document.activeElement).toBe(pendingSubmit.element)

    await pendingForm.trigger('submit')
    expect(fetchMock).toHaveBeenCalledTimes(1)

    resolveRequest(jsonResponse({ error: { code: 'request_failed', message: 'failed' } }, 500))
    await flushPromises()

    expect(wrapper.get('form').attributes('aria-busy')).toBeUndefined()
    expect(wrapper.get('button[type="submit"]').attributes('aria-disabled')).toBeUndefined()
    expect(wrapper.find('[data-testid="submit-status"]').exists()).toBe(false)
    expect(document.activeElement).toBe(wrapper.get('[role="alert"]').element)
    wrapper.unmount()
  })

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
