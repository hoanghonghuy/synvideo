import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { i18n } from '@/locales'
import SceneVideoWorkspaceView from './SceneVideoWorkspaceView.vue'

const mocks = vi.hoisted(() => ({
  listScenePlans: vi.fn(),
  getScenePlan: vi.fn(),
  fetchVideoGenerationOptions: vi.fn(),
  createSceneVideoGeneration: vi.fn(),
  getSceneVideoGeneration: vi.fn(),
  assignPrimaryVisual: vi.fn(),
}))

vi.mock('@/features/scene-plan/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/features/scene-plan/api')>()
  return { ...actual, listScenePlans: mocks.listScenePlans, getScenePlan: mocks.getScenePlan }
})

vi.mock('./api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./api')>()
  return {
    ...actual,
    fetchVideoGenerationOptions: mocks.fetchVideoGenerationOptions,
    createSceneVideoGeneration: mocks.createSceneVideoGeneration,
    getSceneVideoGeneration: mocks.getSceneVideoGeneration,
  }
})

vi.mock('@/features/media/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/features/media/api')>()
  return {
    ...actual,
    assignPrimaryVisual: mocks.assignPrimaryVisual,
    mediaAssetContentURL: (_projectId: string, assetId: string) => `/media/${assetId}`,
  }
})

const scene = {
  key: 'scene-1',
  script_section_key: 'section-1',
  narration: 'Narration',
  visual_instruction: 'Giữ nhân vật ở trung tâm khung hình',
  planned_source_type: 'generated_video',
  expected_duration_seconds: 6,
} as const

const summary = {
  version: 2,
  revision: 3,
  status: 'approved',
  source_script_version: 1,
  source_proposal_version: 1,
  content_locale: 'vi',
  created_at: '2026-09-14T00:00:00Z',
  updated_at: '2026-09-14T00:00:00Z',
  approved_at: '2026-09-14T00:00:00Z',
} as const

const plan = { ...summary, project_id: 'project-1', scenes: [scene] }

const providerOptions = {
  providers: [{
    id: 'provider-raw-id',
    display_name: 'Provider Display',
    models: [{ id: 'model-raw-id', display_name: 'Model Display', min_duration_seconds: 4, max_duration_seconds: 8 }],
  }],
}

async function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/projects/:id/scene-video', component: SceneVideoWorkspaceView }],
  })
  router.push('/projects/project-1/scene-video')
  await router.isReady()
  const wrapper = mount(SceneVideoWorkspaceView, { global: { plugins: [router, i18n] } })
  await flushPromises()
  return wrapper
}

describe('SceneVideoWorkspaceView i18n', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    i18n.global.locale.value = 'vi'
    mocks.listScenePlans.mockResolvedValue([summary])
    mocks.getScenePlan.mockResolvedValue(plan)
    mocks.fetchVideoGenerationOptions.mockResolvedValue(providerOptions)
    mocks.assignPrimaryVisual.mockResolvedValue(undefined)
  })

  it('renders Vietnamese default copy while preserving provider/model identifiers and raw failed job code', async () => {
    localStorage.setItem('synvideo:scene-video-jobs:project-1:2', JSON.stringify({
      'scene-1': [{
        id: 'job-failed', state: 'failed', attempt: 2, max_attempts: 3, error_code: 'VIDEO_PROVIDER_TIMEOUT',
        assigned_primary_visual: false, created_at: '2026-09-14T00:00:00Z', updated_at: '2026-09-14T00:01:00Z',
      }],
    }))

    const wrapper = await mountView()

    expect(wrapper.text()).toContain('Tạo video bằng AI')
    expect(wrapper.text()).toContain('Tạo phương án khác')
    expect(wrapper.text()).toContain('Provider Display')
    expect(wrapper.text()).toContain('Model Display')
    expect(wrapper.text()).toContain('Thất bại · VIDEO_PROVIDER_TIMEOUT')
    expect(wrapper.text()).toContain('Lần thử 2/3')
  })

  it('keeps generation request semantics while localizing controls and progress copy', async () => {
    mocks.createSceneVideoGeneration.mockResolvedValue({
      id: 'job-1', state: 'queued', attempt: 1, max_attempts: 3, assigned_primary_visual: false,
      created_at: '2026-09-14T00:00:00Z', updated_at: '2026-09-14T00:00:00Z',
    })

    const wrapper = await mountView()
    const generateButton = wrapper.findAll('button').find((button) => button.text() === 'Tạo phương án khác')
    expect(generateButton).toBeTruthy()
    await generateButton!.trigger('click')
    await flushPromises()

    expect(mocks.createSceneVideoGeneration).toHaveBeenCalledTimes(1)
    expect(mocks.createSceneVideoGeneration).toHaveBeenCalledWith(
      'project-1',
      2,
      'scene-1',
      expect.objectContaining({
        provider_id: 'provider-raw-id',
        model_id: 'model-raw-id',
        duration_seconds: 4,
        assign_primary_visual: false,
      }),
    )
    expect(wrapper.text()).toContain('Đang chờ')
  })
})
