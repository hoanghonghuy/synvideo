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
    vi.spyOn(mediaApi, 'listMediaAssets').mockResolvedValue({
      assets: [
        {
          id: 'asset-history',
          project_id: 'proj-123',
          kind: 'image',
          origin: 'generated_image',
          mime_type: 'image/png',
          byte_size: 120,
          sha256: 'history',
          metadata: { scene_plan_version: 2, scene_key: 'scene-1', job_id: 'job-history' },
          created_at: '2026-09-03T00:00:00Z',
          updated_at: '2026-09-03T00:00:00Z',
        },
        {
          id: 'asset-other-scene',
          project_id: 'proj-123',
          kind: 'image',
          origin: 'generated_image',
          mime_type: 'image/png',
          byte_size: 120,
          sha256: 'other',
          metadata: { scene_plan_version: 2, scene_key: 'scene-other', job_id: 'job-other' },
          created_at: '2026-09-03T00:00:00Z',
          updated_at: '2026-09-03T00:00:00Z',
        },
      ],
    })
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

  it('shows regeneration history only for the exact approved plan scene', async () => {
    await router.isReady()
    const wrapper = mount(GeneratedImageWorkspaceView, { global: { plugins: [router, i18n] } })
    await flushPromises()

    expect(wrapper.find('[data-testid="history-image-scene-1-asset-history"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="history-image-scene-1-asset-other-scene"]').exists()).toBe(false)
  })

  it('preserves an assignment-failed generated asset in history across deliberate regeneration', async () => {
    const failedAsset = {
      id: 'asset-assignment-failed',
      project_id: 'proj-123',
      kind: 'image' as const,
      origin: 'generated_image' as const,
      mime_type: 'image/png',
      byte_size: 120,
      sha256: 'assignment-failed',
      metadata: { scene_plan_version: 2, scene_key: 'scene-1', job_id: 'job-assignment-failed' },
      created_at: '2026-09-04T00:00:00Z',
      updated_at: '2026-09-04T00:00:00Z',
    }
    vi.spyOn(mediaApi, 'listMediaAssets')
      .mockResolvedValueOnce({ assets: [] })
      .mockResolvedValue({ assets: [failedAsset] })
    vi.spyOn(imageApi, 'createSceneImageGeneration')
      .mockResolvedValueOnce({
        id: '11111111-1111-4111-8111-111111111111',
        state: 'failed',
        attempt: 1,
        max_attempts: 3,
        media_asset_id: failedAsset.id,
        assigned_primary_visual: false,
        error_code: 'assignmentFailed',
        created_at: '2026-09-04T00:00:00Z',
        updated_at: '2026-09-04T00:00:01Z',
      })
      .mockResolvedValueOnce({
        id: '11111111-1111-4111-8111-111111111111',
        state: 'queued',
        attempt: 0,
        max_attempts: 3,
        assigned_primary_visual: false,
        created_at: '2026-09-04T00:01:00Z',
        updated_at: '2026-09-04T00:01:00Z',
      })
    vi.spyOn(imageApi, 'getSceneImageGeneration').mockResolvedValue({
      id: '11111111-1111-4111-8111-111111111111',
      state: 'queued',
      attempt: 0,
      max_attempts: 3,
      assigned_primary_visual: false,
      created_at: '2026-09-04T00:01:00Z',
      updated_at: '2026-09-04T00:01:00Z',
    })

    await router.isReady()
    const wrapper = mount(GeneratedImageWorkspaceView, { global: { plugins: [router, i18n] } })
    await flushPromises()

    await wrapper.get('[data-testid="generate-image-scene-1"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="generated-image-scene-1"]').attributes('src')).toContain(
      failedAsset.id,
    )
    expect(wrapper.find(`[data-testid="history-image-scene-1-${failedAsset.id}"]`).exists()).toBe(true)

    await wrapper.get('[data-testid="generate-image-scene-1"]').trigger('click')
    await flushPromises()

    expect(wrapper.find(`[data-testid="history-image-scene-1-${failedAsset.id}"]`).exists()).toBe(true)
  })
})
