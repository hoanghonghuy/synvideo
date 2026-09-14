import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
import ProviderSettingsView from './ProviderSettingsView.vue'

const fetchMock = vi.fn()

function jsonResponse(data: unknown, status = 200) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(data),
  } as Response)
}

function emptyResponse(status = 204) {
  return Promise.resolve({
    ok: true,
    status,
    json: () => Promise.reject(new Error('no json')),
  } as Response)
}

function createMockProvidersList() {
  return {
    providers: [
      {
        id: 'openai',
        display_name: 'OpenAI',
        configured: true,
        enabled: true,
        has_api_key: true,
        revision: 2,
        models: [
          { id: 'gpt-5-mini', display_name: 'GPT-5 mini', enabled_text: true, enabled_image: false, enabled_tts: false, capabilities: ['text'] },
          { id: 'gpt-4o', display_name: 'GPT-4o', enabled_text: false, enabled_image: false, enabled_tts: false, capabilities: ['text'] },
        ],
        voices: [],
      },
      {
        id: 'openrouter',
        display_name: 'OpenRouter',
        configured: false,
        enabled: false,
        has_api_key: false,
        revision: 0,
        models: [
          { id: 'claude-3-5-sonnet', display_name: 'Claude 3.5 Sonnet', enabled_text: false, enabled_image: false, enabled_tts: false, capabilities: ['text'] },
        ],
        voices: [],
      },
    ],
  }
}

async function mountProviderSettingsView(options: { attachTo?: Element } = {}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/settings/ai-providers',
        name: 'provider-settings',
        component: ProviderSettingsView,
      },
    ],
  })

  await router.push('/settings/ai-providers')
  await router.isReady()

  return mount(ProviderSettingsView, {
    ...options,
    global: {
      plugins: [router, i18n],
    },
  })
}

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
  i18n.global.locale.value = 'vi'
})

describe('ProviderSettingsView', () => {
  it('announces loading without stealing focus and removes the status after load', async () => {
    let resolveLoad: ((value: Response) => void) | undefined
    fetchMock.mockReturnValueOnce(new Promise<Response>((resolve) => {
      resolveLoad = resolve
    }))

    const wrapper = await mountProviderSettingsView({ attachTo: document.body })
    const focusTarget = document.createElement('button')
    document.body.appendChild(focusTarget)
    focusTarget.focus()

    const loadingStatus = wrapper.find('[data-testid="provider-settings-loading-status"]')
    expect(loadingStatus.attributes('role')).toBe('status')
    expect(loadingStatus.attributes('aria-live')).toBe('polite')
    expect(document.activeElement).toBe(focusTarget)

    resolveLoad?.(await jsonResponse(createMockProvidersList()))
    await flushPromises()

    expect(wrapper.find('[data-testid="provider-settings-loading-status"]').exists()).toBe(false)
    expect(document.activeElement).toBe(focusTarget)

    focusTarget.remove()
    wrapper.unmount()
  })

  it('exposes bounded busy and save status semantics only while save is pending', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(createMockProvidersList()))
    const wrapper = await mountProviderSettingsView()
    await flushPromises()

    const openaiCard = wrapper.find('[data-provider-id="openai"]')
    const form = openaiCard.find('form')
    const submit = openaiCard.find('button.btn-primary')
    let resolveSave: ((value: Response) => void) | undefined
    fetchMock.mockReturnValueOnce(new Promise<Response>((resolve) => {
      resolveSave = resolve
    }))

    await form.trigger('submit.prevent')
    await wrapper.vm.$nextTick()

    expect(form.attributes('aria-busy')).toBe('true')
    expect(submit.attributes('disabled')).toBeDefined()
    const saveStatus = submit.find('[data-testid="provider-save-status"]')
    expect(saveStatus.attributes('role')).toBe('status')
    expect(saveStatus.attributes('aria-live')).toBe('polite')
    expect(saveStatus.text()).toContain('Đang lưu')

    resolveSave?.(await jsonResponse(createMockProvidersList().providers[0]))
    await flushPromises()

    expect(form.attributes('aria-busy')).toBeUndefined()
    expect(submit.find('[data-testid="provider-save-status"]').exists()).toBe(false)
    expect(submit.attributes('disabled')).toBeUndefined()
  })

  it('renders list of providers with configured and unconfigured badges', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(createMockProvidersList()))

    const wrapper = await mountProviderSettingsView()
    await flushPromises()

    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).toContain('OpenRouter')
    expect(wrapper.text()).toContain('Đã cấu hình')
    expect(wrapper.text()).toContain('Chưa cấu hình')
    expect(wrapper.text()).toContain('Phiên bản 2')
  })

  it('configures an unconfigured provider with API key and clears key from state', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(createMockProvidersList()))

    const wrapper = await mountProviderSettingsView()
    await flushPromises()

    const openrouterCard = wrapper.find('[data-provider-id="openrouter"]')
    expect(openrouterCard.exists()).toBe(true)

    // Toggle enabled checkbox
    const toggleInput = openrouterCard.find<HTMLInputElement>('input.toggle-checkbox')
    await toggleInput.setValue(true)

    // Select model
    const modelCheckbox = openrouterCard.find<HTMLInputElement>('input[type="checkbox"]:not(.toggle-checkbox)')
    await modelCheckbox.setValue(true)

    // Enter API key
    const keyInput = openrouterCard.find<HTMLInputElement>('input.text-input')
    await keyInput.setValue('sk-openrouter-secret-key-12345')

    // Mock save response
    const updatedOpenrouter = {
      id: 'openrouter',
      display_name: 'OpenRouter',
      configured: true,
      enabled: true,
      has_api_key: true,
      revision: 1,
      models: [
        { id: 'claude-3-5-sonnet', display_name: 'Claude 3.5 Sonnet', enabled_text: true, enabled_image: false, enabled_tts: false, capabilities: ['text'] },
      ],
      voices: [],
    }
    fetchMock.mockResolvedValueOnce(jsonResponse(updatedOpenrouter))

    // Submit form
    await openrouterCard.find('form').trigger('submit.prevent')
    await flushPromises()

    // Assert PUT request sent with key
    expect(fetchMock).toHaveBeenCalledTimes(2)
    const call1 = fetchMock.mock.calls[1] as [string, RequestInit]
    const [putUrl, putOpts] = call1
    expect(putUrl).toBe('/api/v1/ai/provider-settings/openrouter')
    expect(putOpts.method).toBe('PUT')
    const body = JSON.parse(putOpts.body as string)
    expect(body.api_key).toBe('sk-openrouter-secret-key-12345')
    expect(body.enabled).toBe(true)
    expect(body.enabled_text_model_ids).toEqual(['claude-3-5-sonnet'])

    // Assert secret key was immediately cleared from the input element and component memory
    expect(keyInput.element.value).toBe('')

    // Assert badge is now configured
    expect(wrapper.text()).toContain('Đã lưu cấu hình thành công.')
  })

  it('preserves exact API key without trimming whitespace when submitting', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(createMockProvidersList()))

    const wrapper = await mountProviderSettingsView()
    await flushPromises()

    const openrouterCard = wrapper.find('[data-provider-id="openrouter"]')

    const toggleInput = openrouterCard.find<HTMLInputElement>('input.toggle-checkbox')
    await toggleInput.setValue(true)

    const modelCheckbox = openrouterCard.find<HTMLInputElement>('input[type="checkbox"]:not(.toggle-checkbox)')
    await modelCheckbox.setValue(true)

    const keyInput = openrouterCard.find<HTMLInputElement>('input.text-input')
    const keyWithWhitespace = '  sk-key-with-spaces  '
    await keyInput.setValue(keyWithWhitespace)

    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        id: 'openrouter',
        display_name: 'OpenRouter',
        configured: true,
        enabled: true,
        has_api_key: true,
        revision: 1,
        models: [
          { id: 'claude-3-5-sonnet', display_name: 'Claude 3.5 Sonnet', enabled_text: true, enabled_image: false, enabled_tts: false, capabilities: ['text'] },
        ],
      }),
    )

    await openrouterCard.find('form').trigger('submit.prevent')
    await flushPromises()

    const call = fetchMock.mock.calls[1] as [string, RequestInit]
    const body = JSON.parse(call[1].body as string)
    expect(body.api_key).toBe('  sk-key-with-spaces  ')
  })

  it('updates configured provider preserving key when input is blank', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(createMockProvidersList()))

    const wrapper = await mountProviderSettingsView()
    await flushPromises()

    const openaiCard = wrapper.find('[data-provider-id="openai"]')
    expect(openaiCard.exists()).toBe(true)

    const keyInput = openaiCard.find<HTMLInputElement>('input.text-input')
    expect(keyInput.element.placeholder).toContain('Giữ nguyên khóa hiện tại')

    // Mock save response with revision 3
    const updatedOpenAI = {
      id: 'openai',
      display_name: 'OpenAI',
      configured: true,
      enabled: false,
      has_api_key: true,
      revision: 3,
      models: [
        { id: 'gpt-5-mini', display_name: 'GPT-5 mini', enabled_text: true, enabled_image: false, enabled_tts: false, capabilities: ['text'] },
        { id: 'gpt-4o', display_name: 'GPT-4o', enabled_text: false, enabled_image: false, enabled_tts: false, capabilities: ['text'] },
      ],
    }
    fetchMock.mockResolvedValueOnce(jsonResponse(updatedOpenAI))

    // Toggle enabled off
    const toggleInput = openaiCard.find<HTMLInputElement>('input.toggle-checkbox')
    await toggleInput.setValue(false)

    // Submit form without typing new key
    await openaiCard.find('form').trigger('submit.prevent')
    await flushPromises()

    const updateCall = fetchMock.mock.calls[1] as [string, RequestInit]
    const [putUrl, putOpts] = updateCall
    expect(putUrl).toBe('/api/v1/ai/provider-settings/openai')
    const body = JSON.parse(putOpts.body as string)
    expect(body.revision).toBe(2)
    expect(body.enabled).toBe(false)
    expect(body.api_key).toBeUndefined()
  })

  it('requires in-app confirmation, supports cancel, and deletes only after confirm', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(createMockProvidersList()))

    const wrapper = await mountProviderSettingsView({ attachTo: document.body })
    await flushPromises()

    let openaiCard = wrapper.find('[data-provider-id="openai"]')
    const deleteBtn = openaiCard.find('button.btn-danger')
    expect(deleteBtn.exists()).toBe(true)

    await deleteBtn.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(openaiCard.find('[data-testid="provider-delete-confirmation"]').exists()).toBe(true)
    expect(openaiCard.text()).toContain('Thao tác này không thể hoàn tác')
    expect((document.activeElement as HTMLElement | null)?.id).toBe('confirm-provider-delete-openai')

    const confirmation = openaiCard.find('[data-testid="provider-delete-confirmation"]')
    await confirmation.findAll('button')[1]!.trigger('click')
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(openaiCard.find('[data-testid="provider-delete-confirmation"]').exists()).toBe(false)
    expect((document.activeElement as HTMLElement | null)?.id).toBe('delete-provider-openai')

    await openaiCard.find('button.btn-danger').trigger('click')
    await flushPromises()

    fetchMock.mockResolvedValueOnce(emptyResponse(204))
    fetchMock.mockResolvedValueOnce(jsonResponse({
      providers: [
        {
          id: 'openai',
          display_name: 'OpenAI',
          configured: false,
          enabled: false,
          has_api_key: false,
          revision: 0,
          models: [
            { id: 'gpt-5-mini', display_name: 'GPT-5 mini', enabled_text: false, enabled_image: false, enabled_tts: false, capabilities: ['text'] },
          ],
          voices: [],
        },
      ],
    }))

    openaiCard = wrapper.find('[data-provider-id="openai"]')
    await openaiCard.find('#confirm-provider-delete-openai').trigger('click')
    await flushPromises()

    const deleteCall = fetchMock.mock.calls[1] as [string, RequestInit]
    expect(deleteCall[0]).toBe('/api/v1/ai/provider-settings/openai?revision=2')
    expect(deleteCall[1]?.method).toBe('DELETE')
    expect(wrapper.find('[data-testid="provider-delete-confirmation"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Đã xóa cấu hình nhà cung cấp.')

    wrapper.unmount()
  })

  it('handles stale revision conflict by refetching', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(createMockProvidersList()))

    const wrapper = await mountProviderSettingsView()
    await flushPromises()

    const openaiCard = wrapper.find('[data-provider-id="openai"]')

    await openaiCard.find('button.btn-danger').trigger('click')
    await flushPromises()
    expect(openaiCard.find('[data-testid="provider-delete-confirmation"]').exists()).toBe(true)

    // 409 conflict
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ error: { code: 'STALE_REVISION', message: 'Stale revision' } }, 409),
    )
    // Refetch after conflict
    fetchMock.mockResolvedValueOnce(jsonResponse(createMockProvidersList()))

    await openaiCard.find('form').trigger('submit.prevent')
    await flushPromises()

    const updatedCard = wrapper.find('[data-provider-id="openai"]')
    expect(updatedCard.text()).toContain('Cấu hình đã thay đổi trên máy chủ')
    expect(updatedCard.find('[data-testid="provider-delete-confirmation"]').exists()).toBe(false)
  })
})
