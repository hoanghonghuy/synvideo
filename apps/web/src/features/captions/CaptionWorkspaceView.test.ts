import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { i18n } from '@/locales'
import CaptionWorkspaceView from './CaptionWorkspaceView.vue'

const mocks = vi.hoisted(() => ({
  getCaptions: vi.fn(),
  listCaptionHistory: vi.fn(),
  deriveCaptions: vi.fn(),
  updateCaptions: vi.fn(),
  rebuildCaptions: vi.fn(),
}))

vi.mock('./api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./api')>()
  return { ...actual, ...mocks }
})

const view = {
  id: 'caption-1', project_id: 'project-1', scene_plan_version: 1, scene_key: 'scene-1', revision: 2,
  source_binding_id: 'binding-1', source_asset_id: 'asset-123456789', source_duration_ms: 2400,
  segments: [{ id: 'seg-1', text: 'Xin chào', start_ms: 0, end_ms: 1200 }],
  style: { alignment: 'center', position: 'bottom', size: 'medium', weight: 'normal' },
  created_at: '2026-09-14T00:00:00Z', state: 'CURRENT',
} as const

async function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/projects/:id/captions', component: CaptionWorkspaceView }],
  })
  router.push('/projects/project-1/captions?version=1&scene=scene-1')
  await router.isReady()
  const wrapper = mount(CaptionWorkspaceView, { global: { plugins: [router, i18n] } })
  await flushPromises()
  return wrapper
}

describe('CaptionWorkspaceView i18n', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    i18n.global.locale.value = 'vi'
    mocks.getCaptions.mockResolvedValue(view)
    mocks.listCaptionHistory.mockResolvedValue([view])
  })

  it('renders Vietnamese default workspace copy while preserving authoritative state and lineage values', async () => {
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('Không gian căn thời gian phụ đề')
    expect(wrapper.text()).toContain('Các đoạn')
    expect(wrapper.text()).toContain('Lịch sử revision')
    expect(wrapper.text()).toContain('CURRENT')
    expect(wrapper.text()).toContain('revision 2')
    expect(wrapper.text()).toContain('source 2400 ms')
    expect(wrapper.text()).toContain('asset-12')
    expect(mocks.getCaptions).toHaveBeenCalledWith('project-1', 1, 'scene-1')
  })
})
