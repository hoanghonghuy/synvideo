import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
import CreativeBriefView from '@/features/creative-brief/CreativeBriefView.vue'
import CreativeProposalView from '@/features/creative-proposal/CreativeProposalView.vue'
import ProjectDetailView from './ProjectDetailView.vue'

const fetchMock = vi.fn()

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
  i18n.global.locale.value = 'vi'
})

describe('ProjectDetailView', () => {
  const project = {
    id: '11111111-1111-4111-8111-111111111111',
    title: 'Video chi tiet',
    description: 'Mo ta du an',
    content_format: 'short',
    aspect_ratio: '9:16',
    target_duration_seconds: 60,
    locale: 'vi',
    status: 'active',
    created_at: '2026-08-31T08:00:00Z',
    updated_at: '2026-08-31T08:30:00Z',
  }

  it('renders project detail with localized date and allows updates', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(project))

    const wrapper = await mountDetailView()

    await flushPromises()

    expect(wrapper.text()).toContain('Video chi tiet')
    expect(wrapper.text()).toContain('Cập nhật lần cuối:')
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/11111111-1111-4111-8111-111111111111',
      expect.anything(),
    )
  })

  it('formats updated time using the active i18n locale', async () => {
    i18n.global.setLocaleMessage('en', i18n.global.getLocaleMessage('vi'))
    i18n.global.locale.value = 'en'
    fetchMock.mockResolvedValueOnce(jsonResponse(project))

    const wrapper = await mountDetailView()

    await flushPromises()

    const expectedEnglishDate = i18n.global.d(new Date(project.updated_at), 'long')
    const hardCodedVietnameseDate = new Date(project.updated_at).toLocaleString('vi-VN')
    expect(wrapper.text()).toContain(expectedEnglishDate)
    expect(wrapper.text()).not.toContain(hardCodedVietnameseDate)
  })

  it('renders creative workspace links and navigates to their production routes', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(
        jsonResponse(
          { error: { code: 'creative_brief_not_found', message: 'Creative brief was not found.' } },
          404,
        ),
      )
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse(project))
      .mockResolvedValueOnce(jsonResponse([]))
      .mockResolvedValueOnce(jsonResponse({ providers: [] }))

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/projects', component: { template: '<div />' } },
        { path: '/projects/:id', component: ProjectDetailView },
        { path: '/projects/:id/creative-brief', component: CreativeBriefView },
        { path: '/projects/:id/creative-proposal', component: CreativeProposalView },
        { path: '/projects/:id/script', component: { template: '<div>Script workspace</div>' } },
      ],
    })
    router.push(`/projects/${project.id}`)
    await router.isReady()

    const wrapper = mount({ template: '<RouterView />' }, { global: { plugins: [router, i18n] } })
    await flushPromises()

    const links = wrapper.findAll('.workspace-links a')
    const briefLink = links.find((link) => link.attributes('href') === `/projects/${project.id}/creative-brief`)
    const proposalLink = links.find((link) => link.attributes('href') === `/projects/${project.id}/creative-proposal`)
    const scriptLink = links.find((link) => link.attributes('href') === `/projects/${project.id}/script`)
    expect(briefLink?.text()).toContain('Mở Creative Brief')
    expect(proposalLink?.text()).toContain('Mở AI Proposal')
    expect(scriptLink?.text()).toContain('Mở Script')

    await briefLink?.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(router.currentRoute.value.path).toBe(`/projects/${project.id}/creative-brief`)
    expect(wrapper.text()).toContain('Bản nháp mới')

    router.push(`/projects/${project.id}`)
    await flushPromises()
    await flushPromises()

    const proposalLinkAfterReturn = wrapper
      .findAll('.workspace-links a')
      .find((link) => link.attributes('href') === `/projects/${project.id}/creative-proposal`)
    await proposalLinkAfterReturn?.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(router.currentRoute.value.path).toBe(`/projects/${project.id}/creative-proposal`)
    expect(wrapper.text()).toContain('Chưa có AI Proposal')
  })
})

async function mountDetailView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/projects', component: { template: '<div />' } },
      { path: '/projects/:id', component: ProjectDetailView },
      { path: '/projects/:id/creative-brief', component: { template: '<div />' } },
      { path: '/projects/:id/creative-proposal', component: { template: '<div />' } },
      { path: '/projects/:id/script', component: { template: '<div />' } },
    ],
  })
  router.push('/projects/11111111-1111-4111-8111-111111111111')
  await router.isReady()

  return mount(ProjectDetailView, {
    global: {
      plugins: [i18n, router],
    },
  })
}

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  }
}

function flushPromises() {
  return new Promise((resolve) => window.setTimeout(resolve))
}
