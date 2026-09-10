import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  completeSignInFromCallback: vi.fn(),
  pendingSignInReturnTo: vi.fn(() => '/projects/project-1/scene-editor?tab=timeline#clip-3'),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/auth/callback?error=access_denied' }),
  useRouter: () => ({ replace: mocks.replace }),
}))

vi.mock('@/auth/config', () => ({
  isOidcConfigured: () => true,
}))

vi.mock('@/auth/oidc', () => ({
  completeSignInFromCallback: mocks.completeSignInFromCallback,
  pendingSignInReturnTo: mocks.pendingSignInReturnTo,
}))

import AuthCallbackView from './AuthCallbackView.vue'

describe('AuthCallbackView', () => {
  afterEach(() => {
    mocks.replace.mockReset()
    mocks.completeSignInFromCallback.mockReset()
    mocks.pendingSignInReturnTo.mockReset()
    mocks.pendingSignInReturnTo.mockReturnValue('/projects/project-1/scene-editor?tab=timeline#clip-3')
    document.body.innerHTML = ''
  })

  it('turns a failed callback into an actionable, focused recovery state and preserves destination', async () => {
    mocks.completeSignInFromCallback.mockRejectedValue(new Error('Provider rejected sign-in.'))

    const wrapper = mount(AuthCallbackView, { attachTo: document.body })
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
