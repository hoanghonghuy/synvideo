import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import { i18n } from '@/locales'

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  isOidcConfigured: vi.fn(() => true),
  completeSignInFromCallback: vi.fn(),
  pendingSignInReturnTo: vi.fn(() => '/projects/project-1/scene-editor?tab=timeline#clip-3'),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/auth/callback?error=access_denied' }),
  useRouter: () => ({ replace: mocks.replace }),
}))

vi.mock('@/auth/config', () => ({
  isOidcConfigured: mocks.isOidcConfigured,
}))

vi.mock('@/auth/oidc', () => ({
  completeSignInFromCallback: mocks.completeSignInFromCallback,
  pendingSignInReturnTo: mocks.pendingSignInReturnTo,
}))

import AuthCallbackView from './AuthCallbackView.vue'

describe('AuthCallbackView', () => {
  afterEach(() => {
    mocks.replace.mockReset()
    mocks.isOidcConfigured.mockReset()
    mocks.isOidcConfigured.mockReturnValue(true)
    mocks.completeSignInFromCallback.mockReset()
    mocks.pendingSignInReturnTo.mockReset()
    mocks.pendingSignInReturnTo.mockReturnValue('/projects/project-1/scene-editor?tab=timeline#clip-3')
    document.body.innerHTML = ''
  })

  it('renders localized unconfigured callback failure and focuses recovery heading', async () => {
    mocks.isOidcConfigured.mockReturnValue(false)

    const wrapper = mount(AuthCallbackView, { attachTo: document.body, global: { plugins: [i18n] } })
    await vi.waitFor(() => expect(wrapper.get('h1').text()).toBe('Đăng nhập chưa hoàn tất'))

    expect(wrapper.get('p').text()).toBe('Không thể hoàn tất đăng nhập vì OIDC chưa được cấu hình.')
    expect(wrapper.get('button.recovery-action').text()).toBe('Thử đăng nhập lại')
    expect(document.activeElement).toBe(wrapper.get('h1').element)
    expect(mocks.completeSignInFromCallback).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('renders localized loading copy while callback is pending', async () => {
    mocks.completeSignInFromCallback.mockReturnValue(new Promise(() => {}))

    const wrapper = mount(AuthCallbackView, { attachTo: document.body, global: { plugins: [i18n] } })
    expect(wrapper.get('[role="status"] h1').text()).toBe('Đang hoàn tất đăng nhập')
    expect(wrapper.get('[role="status"] p').text()).toBe('Vui lòng chờ trong giây lát…')
    wrapper.unmount()
  })

  it('turns a failed callback into an actionable, focused recovery state and preserves destination', async () => {
    mocks.completeSignInFromCallback.mockRejectedValue(new Error('Provider rejected sign-in.'))

    const wrapper = mount(AuthCallbackView, { attachTo: document.body, global: { plugins: [i18n] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('Đăng nhập chưa hoàn tất'))

    const heading = wrapper.get('h1')
    expect(document.activeElement).toBe(heading.element)
    expect(wrapper.text()).toContain('Provider rejected sign-in.')
    expect(mocks.pendingSignInReturnTo).toHaveBeenCalledOnce()

    const retry = wrapper.get('button.recovery-action')
    expect(retry.text()).toBe('Thử đăng nhập lại')
    await retry.trigger('click')
    await nextTick()

    expect(mocks.replace).toHaveBeenCalledWith({
      path: '/sign-in',
      query: {
        reason: 'callback-failed',
        returnTo: '/projects/project-1/scene-editor?tab=timeline#clip-3',
      },
    })

    wrapper.unmount()
  })
})
