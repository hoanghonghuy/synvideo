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
    expect(signOutButton.text()).toBe('Sign out')
    await signOutButton.trigger('click')
    await router.isReady()

    expect(getAccessToken()).toBeNull()
    expect(router.currentRoute.value.path).toBe('/sign-in')
    expect(router.currentRoute.value.query.reason).toBe('signed-out')
  })
})
