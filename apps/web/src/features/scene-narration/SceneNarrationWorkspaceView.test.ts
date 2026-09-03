import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createI18n } from 'vue-i18n'

import SceneNarrationWorkspaceView from './SceneNarrationWorkspaceView.vue'
import * as projectApi from '@/api/projects'
import * as scenePlanApi from '@/features/scene-plan/api'
import * as narrationApi from './api'
import viMessages from '@/locales/vi'

const i18n = createI18n({
  legacy: false,
  locale: 'vi',
  messages: {
    vi: viMessages,
  },
})

describe('SceneNarrationWorkspaceView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    vi.restoreAllMocks()
    router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/projects/:id/narration',
          name: 'scene-narration',
          component: SceneNarrationWorkspaceView,
        },
      ],
    })
    router.push('/projects/proj-123/narration')
  })

  it('renders notice when no approved scene plan exists', async () => {
    vi.spyOn(projectApi, 'getProject').mockResolvedValue({
      id: 'proj-123',
      title: 'Test Project',
      description: 'Test description',
      content_format: 'short',
      aspect_ratio: '16:9',
      target_duration_seconds: 30,
      locale: 'vi',
      status: 'active',
      created_at: '2026-09-01T00:00:00Z',
      updated_at: '2026-09-01T00:00:00Z',
    })
    vi.spyOn(scenePlanApi, 'listScenePlans').mockResolvedValue([
      {
        version: 1,
        revision: 1,
        status: 'draft',
        source_script_version: 1,
        source_proposal_version: 1,
        content_locale: 'vi-VN',
        created_at: '2026-09-01T00:00:00Z',
        updated_at: '2026-09-01T00:00:00Z',
        approved_at: null,
      },
    ])
    vi.spyOn(narrationApi, 'fetchTTSOptions').mockResolvedValue({
      providers: [],
    })

    await router.isReady()
    const wrapper = mount(SceneNarrationWorkspaceView, {
      global: {
        plugins: [router, i18n],
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Chưa có Kế hoạch phân cảnh được phê duyệt')
  })

  it('renders scenes and allows generating narration audio', async () => {
    vi.spyOn(projectApi, 'getProject').mockResolvedValue({
      id: 'proj-123',
      title: 'Test Project',
      description: 'Test description',
      content_format: 'short',
      aspect_ratio: '16:9',
      target_duration_seconds: 30,
      locale: 'vi',
      status: 'active',
      created_at: '2026-09-01T00:00:00Z',
      updated_at: '2026-09-01T00:00:00Z',
    })
    vi.spyOn(scenePlanApi, 'listScenePlans').mockResolvedValue([
      {
        version: 1,
        revision: 1,
        status: 'approved',
        source_script_version: 1,
        source_proposal_version: 1,
        content_locale: 'vi-VN',
        created_at: '2026-09-01T00:00:00Z',
        updated_at: '2026-09-01T00:00:00Z',
        approved_at: '2026-09-01T00:00:00Z',
      },
    ])
    vi.spyOn(scenePlanApi, 'getScenePlan').mockResolvedValue({
      project_id: 'proj-123',
      version: 1,
      revision: 1,
      status: 'approved',
      content_locale: 'vi-VN',
      source_script_version: 1,
      source_proposal_version: 1,
      created_at: '2026-09-01T00:00:00Z',
      updated_at: '2026-09-01T00:00:00Z',
      approved_at: '2026-09-01T00:00:00Z',
      scenes: [
        {
          key: 'sc-1',
          script_section_key: 'intro',
          narration: 'Xin chào các bạn đã đến với kênh.',
          visual_instruction: 'Toàn cảnh mở đầu',
          planned_source_type: 'generated_image',
          expected_duration_seconds: 5,
        },
        {
          key: 'sc-2',
          script_section_key: 'main',
          narration: 'Hôm nay chúng ta cùng khám phá điều kỳ diệu.',
          visual_instruction: 'Cận cảnh chi tiết',
          planned_source_type: 'generated_image',
          expected_duration_seconds: 5,
        },
      ],
    })
    vi.spyOn(narrationApi, 'fetchTTSOptions').mockResolvedValue({
      providers: [
        {
          id: 'openai',
          display_name: 'OpenAI',
          models: [{ id: 'tts-1', display_name: 'TTS-1' }],
          voices: [{ id: 'voice-nova', display_name: 'Nova' }],
        },
      ],
    })
    vi.spyOn(narrationApi, 'listSceneNarrations').mockResolvedValue([
      {
        scene_key: 'sc-1',
        role: 'narration',
      },
      {
        scene_key: 'sc-2',
        role: 'narration',
      },
    ])

    const createSpy = vi.spyOn(narrationApi, 'createSceneNarrationGeneration').mockResolvedValue({
      id: 'job-1',
      state: 'queued',
      attempt: 0,
      max_attempts: 3,
      duration_seconds: 0,
      assigned_narration: true,
      created_at: '2026-09-01T00:00:00Z',
      updated_at: '2026-09-01T00:00:00Z',
    })

    vi.spyOn(narrationApi, 'getSceneNarrationGeneration').mockResolvedValue({
      id: 'job-1',
      state: 'succeeded',
      attempt: 1,
      max_attempts: 3,
      media_asset_id: 'asset-999',
      duration_seconds: 4.8,
      assigned_narration: true,
      created_at: '2026-09-01T00:00:00Z',
      updated_at: '2026-09-01T00:00:00Z',
    })

    await router.isReady()
    const wrapper = mount(SceneNarrationWorkspaceView, {
      global: {
        plugins: [router, i18n],
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Xin chào các bạn đã đến với kênh.')
    expect(wrapper.text()).toContain('Hôm nay chúng ta cùng khám phá điều kỳ diệu.')

    // Find and click generate button for first scene
    const generateBtn = wrapper.find('[data-testid="generate-narration-sc-1"]')
    expect(generateBtn.exists()).toBe(true)
    await generateBtn.trigger('click')
    await flushPromises()

    expect(createSpy).toHaveBeenCalled()
  })
})
