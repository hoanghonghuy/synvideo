import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { clearAccessToken, getAccessToken, setAccessToken } from '@/auth/session'
import App from './App.vue'
import { i18n } from './locales'
import { router } from './router'

describe('App', () => {
  afterEach(() => {
    clearAccessToken()
    vi.unstubAllEnvs()
  })

  it('renders localized navigation through the router shell', async () => {
    router.push('/')
    await router.isReady()

    const wrapper = mount(App, {
      global: {
        plugins: [router, i18n],
      },
    })

    expect(wrapper.text()).toContain('SynVideo')
    expect(wrapper.text()).toContain('Trang chủ')
    expect(wrapper.text()).toContain('tài nguyên ngôn ngữ tiếng Việt')
  })

  it('provides a first-focusable skip link that moves focus to main content', async () => {
    await router.push('/')
    await router.isReady()

    const wrapper = mount(App, {
      attachTo: document.body,
      global: {
        plugins: [router, i18n],
      },
    })

    const skipLink = wrapper.get('a.skip-link')
    const nav = wrapper.get('nav')
    const main = wrapper.get('#main-content')

    expect(skipLink.attributes('href')).toBe('#main-content')
    expect(skipLink.element.compareDocumentPosition(nav.element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(main.attributes('tabindex')).toBe('-1')

    await skipLink.trigger('click')
    expect(document.activeElement).toBe(main.element)

    wrapper.unmount()
  })

  it('moves focus to main content after a client-side route transition without stealing initial focus', async () => {
    await router.push('/')
    await router.isReady()

    const wrapper = mount(App, {
      attachTo: document.body,
      global: {
        plugins: [router, i18n],
      },
    })

    const main = wrapper.get('#main-content')
    expect(document.activeElement).not.toBe(main.element)

    await router.push('/status')
    await vi.waitFor(() => {
      expect(wrapper.text()).toContain('Kiểm tra hệ thống')
      expect(document.activeElement).toBe(main.element)
    })

    wrapper.unmount()
  })

  it('signs out from the shell, clears memory auth, and returns to the safe sign-in screen', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    setAccessToken('memory-only-token')
    await router.push('/status')
    await router.isReady()

    const wrapper = mount(App, {
      global: {
        plugins: [router, i18n],
      },
    })

    const signOutButton = wrapper.get('button.nav-auth-action')
    expect(signOutButton.text()).toBe('Đăng xuất')
    await signOutButton.trigger('click')

    expect(getAccessToken()).toBeNull()
    await vi.waitFor(() => {
      expect(router.currentRoute.value.path).toBe('/sign-in')
    })
    expect(router.currentRoute.value.query.reason).toBe('signed-out')
  })
})
