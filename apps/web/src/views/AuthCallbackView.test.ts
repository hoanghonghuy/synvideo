import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

const replace = vi.fn()
const completeSignInFromCallback = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/auth/callback?error=access_denied' }),
  useRouter: () => ({ replace }),
}))

vi.mock('@/auth/config', () => ({
  isOidcConfigured: () => true,
}))

vi.mock('@/auth/oidc', () => ({
  completeSignInFromCallback,
}))

import AuthCallbackView from './AuthCallbackView.vue'

describe('AuthCallbackView', () => {
  afterEach(() => {
    replace.mockReset()
    completeSignInFromCallback.mockReset()
  })

  it('turns a failed callback into an actionable, focused recovery state', async () => {
    completeSignInFromCallback.mockRejectedValue(new Error('Provider rejected sign-in.'))

    const wrapper = mount(AuthCallbackView, { attachTo: document.body })
    await vi.waitFor(() => expect(wrapper.text()).toContain('Đăng nhập chưa hoàn tất'))

    const heading = wrapper.get('h1')
    expect(document.activeElement).toBe(heading.element)
    expect(wrapper.text()).toContain('Provider rejected sign-in.')

    const retry = wrapper.get('button.recovery-action')
    expect(retry.text()).toBe('Thử đăng nhập lại')
    await retry.trigger('click')
    await nextTick()

    expect(replace).toHaveBeenCalledWith({
      path: '/sign-in',
      query: { reason: 'callback-failed' },
    })

    wrapper.unmount()
  })
})
