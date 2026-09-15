import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { i18n } from '@/locales'

const mocks = vi.hoisted(() => ({
  beginSignIn: vi.fn(),
  isOidcConfigured: vi.fn(() => true),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { reason: 'session-expired', returnTo: '/projects' } }),
}))

vi.mock('@/auth/config', () => ({
  isOidcConfigured: mocks.isOidcConfigured,
}))

vi.mock('@/auth/oidc', () => ({
  beginSignIn: mocks.beginSignIn,
  sanitizeReturnTo: (value: string) => value,
}))

import SignInView from './SignInView.vue'

describe('SignInView localization', () => {
  afterEach(() => {
    mocks.beginSignIn.mockReset()
    mocks.isOidcConfigured.mockReset()
    mocks.isOidcConfigured.mockReturnValue(true)
    document.body.innerHTML = ''
  })

  it('renders locale-backed session recovery copy and action', async () => {
    const wrapper = mount(SignInView, { attachTo: document.body, global: { plugins: [i18n] } })

    expect(wrapper.get('.eyebrow').text()).toBe('Tài khoản SynVideo')
    expect(wrapper.get('h1').text()).toBe('Phiên đăng nhập đã hết hạn')
    expect(wrapper.get('.body-copy').text()).toContain('Đăng nhập lại để quay về đúng không gian làm việc')
    expect(wrapper.get('button').text()).toBe('Đăng nhập')

    await wrapper.get('button').trigger('click')
    expect(mocks.beginSignIn).toHaveBeenCalledWith('/projects')
    wrapper.unmount()
  })

  it('announces a bounded pending redirect state without moving focus', async () => {
    let rejectSignIn!: (reason?: unknown) => void
    mocks.beginSignIn.mockImplementation(() => new Promise<void>((_resolve, reject) => {
      rejectSignIn = reject
    }))
    const wrapper = mount(SignInView, { attachTo: document.body, global: { plugins: [i18n] } })
    const button = wrapper.get('button')
    button.element.focus()

    await button.trigger('click')

    expect(wrapper.get('.auth-panel').attributes('aria-busy')).toBe('true')
    expect(wrapper.get('[role="status"]').text()).toBe('Đang mở đăng nhập…')
    expect(wrapper.get('[role="status"]').attributes('aria-live')).toBe('polite')
    expect(wrapper.get('[role="status"]').attributes('aria-atomic')).toBe('true')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(document.activeElement).toBe(button.element)

    rejectSignIn('network unavailable')
    await vi.waitFor(() => expect(wrapper.find('[role="status"]').exists()).toBe(false))
    expect(wrapper.get('.auth-panel').attributes('aria-busy')).toBeUndefined()
    expect(document.activeElement).toBe(wrapper.get('h1').element)
    wrapper.unmount()
  })

  it('renders localized local start failure and returns focus to the recovery heading', async () => {
    mocks.beginSignIn.mockRejectedValue('network unavailable')
    const wrapper = mount(SignInView, { attachTo: document.body, global: { plugins: [i18n] } })

    await wrapper.get('button').trigger('click')
    await vi.waitFor(() => expect(wrapper.get('h1').text()).toBe('Đăng nhập cần được xử lý'))

    expect(wrapper.get('.body-copy').text()).toBe('Không thể bắt đầu đăng nhập.')
    expect(wrapper.get('button').text()).toBe('Thử đăng nhập lại')
    expect(document.activeElement).toBe(wrapper.get('h1').element)
    wrapper.unmount()
  })

  it('renders localized unconfigured recovery copy without exposing a sign-in action', async () => {
    mocks.isOidcConfigured.mockReturnValue(false)
    const wrapper = mount(SignInView, { attachTo: document.body, global: { plugins: [i18n] } })

    await vi.waitFor(() => expect(wrapper.get('h1').text()).toBe('Đăng nhập cần được xử lý'))
    expect(wrapper.get('.body-copy').text()).toBe('Môi trường này chưa được cấu hình đăng nhập.')
    expect(wrapper.find('button').exists()).toBe(false)
    wrapper.unmount()
  })

})
