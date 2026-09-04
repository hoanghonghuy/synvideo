import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'

import * as projectApi from '@/api/projects'
import * as mediaApi from '@/features/media/api'
import * as scenePlanApi from '@/features/scene-plan/api'
import viMessages from '@/locales/vi'
import GeneratedImageWorkspaceView from './GeneratedImageWorkspaceView.vue'
import * as imageApi from './api'

const i18n = createI18n({
  legacy: false,
  locale: 'vi',
  messages: { vi: viMessages },
})

const project = {
  id: 'proj-123',
  title: 'Test Project',
  description: 'Test description',
  content_format: 'short' as const,
  aspect_ratio: '16:9' as const,
  target_duration_seconds: 30,
  locale: 'vi' as const,
  status: 'active' as const,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
}

const approvedPlan = {
  project_id: 'proj-123',
  version: 2,
  revision: 1,
  status: 'approved' as const,
  content_locale: 'vi-VN',
  source_script_version: 1,
  source_proposal_version: 1,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
  approved_at: '2026-09-01T00:00:00Z',
  scenes: [
    {
      key: 'scene-1',
      script_section_key: 'intro',
      narration: 'Mở đầu',
      visual_instruction: 'A lighthouse at dusk',
      planned_source_type: 'generated_image' as const,
      expected_duration_seconds: 5,
    },
  ],
}

describe('GeneratedImageWorkspaceView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    vi.restoreAllMocks()
    sessionStorage.clear()
    vi.stubGlobal('crypto', { randomUUID: () => '11111111-1111-4111-8111-111111111111' })
    router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/projects/:id/images',
          name: 'generated-images',
          component: GeneratedImageWorkspaceView,
        },
      ],
    })
    router.push('/projects/proj-123/images')

    vi.spyOn(projectApi, 'getProject').mockResolvedValue(project)
    vi.spyOn(scenePlanApi, 'listScenePlans').mockResolvedValue([
      {
        version: 2,
        revision: 1,
        status: 'approved',
        content_locale: 'vi-VN',
        source_script_version: 1,
        source_proposal_version: 1,
        created_at: '2026-09-01T00:00:00Z',
        updated_at: '2026-09-01T00:00:00Z',
        approved_at: '2026-09-01T00:00:00Z',
      },
    ])
    vi.spyOn(scenePlanApi, 'getScenePlan').mockResolvedValue(approvedPlan)
    vi.spyOn(imageApi, 'fetchImageGenerationOptions').mockResolvedValue({
      providers: [
        {
          id: 'openai',
          display_name: 'OpenAI',
          models: [{ id: 'image-1', display_name: 'Image 1' }],
        },
      ],
    })
    vi.spyOn(mediaApi, 'listSceneMediaBindings').mockResolvedValue([
      { scene_key: 'scene-1', role: 'primary_visual' },
    ])
  })

  it('submits the scene-local edited prompt and previews the exact succeeded asset', async () => {
    const createSpy = vi.spyOn(imageApi, 'createSceneImageGeneration').mockResolvedValue({
      id: '11111111-1111-4111-8111-111111111111',
      state: 'queued',
      attempt: 0,
      max_attempts: 3,
      assigned_primary_visual: false,
      created_at: '2026-09-04T00:00:00Z',
      updated_at: '2026-09-04T00:00:00Z',
    })
    vi.spyOn(imageApi, 'getSceneImageGeneration').mockResolvedValue({
      id: '11111111-1111-4111-8111-111111111111',
      state: 'succeeded',
      attempt: 1,
      max_attempts: 3,
      media_asset_id: 'asset-99',
      assigned_primary_visual: false,
      created_at: '2026-09-04T00:00:00Z',
      updated_at: '2026-09-04T00:00:01Z',
    })

    await router.isReady()
    const wrapper = mount(GeneratedImageWorkspaceView, { global: { plugins: [router, i18n] } })
    await flushPromises()

    const prompt = wrapper.get('[data-testid="image-prompt-scene-1"]')
    expect((prompt.element as HTMLTextAreaElement).value).toBe('A lighthouse at dusk')
    await prompt.setValue('Edited lighthouse prompt')
    await wrapper.get('[data-testid="generate-image-scene-1"]').trigger('click')
    await flushPromises()

    expect(createSpy).toHaveBeenCalledWith('proj-123', 2, 'scene-1', {
      request_id: '11111111-1111-4111-8111-111111111111',
      provider_id: 'openai',
      model_id: 'image-1',
      prompt: 'Edited lighthouse prompt',
      assign_primary_visual: false,
    })
    expect(wrapper.get('[data-testid="generated-image-scene-1"]').attributes('src')).toBe(
      '/api/v1/projects/proj-123/media-assets/asset-99/content',
    )
  })

  it('recovers a persisted in-flight request without submitting paid work again', async () => {
    sessionStorage.setItem(
      'synvideo:generated-image:proj-123:2:scene-1',
      JSON.stringify({ requestId: 'existing-request' }),
    )
    const createSpy = vi.spyOn(imageApi, 'createSceneImageGeneration')
    const getSpy = vi.spyOn(imageApi, 'getSceneImageGeneration').mockResolvedValue({
      id: 'existing-request',
      state: 'succeeded',
      attempt: 1,
      max_attempts: 3,
      media_asset_id: 'asset-existing',
      assigned_primary_visual: false,
      created_at: '2026-09-04T00:00:00Z',
      updated_at: '2026-09-04T00:00:01Z',
    })

    await router.isReady()
    const wrapper = mount(GeneratedImageWorkspaceView, { global: { plugins: [router, i18n] } })
    await flushPromises()

    expect(createSpy).not.toHaveBeenCalled()
    expect(getSpy).toHaveBeenCalledWith('proj-123', 'existing-request')
    expect(wrapper.get('[data-testid="generated-image-scene-1"]').attributes('src')).toContain(
      'asset-existing',
    )
  })
})
