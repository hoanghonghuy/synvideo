import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import App from './App.vue'
import { i18n } from './locales'
import { router } from './router'

describe('App', () => {
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
})
