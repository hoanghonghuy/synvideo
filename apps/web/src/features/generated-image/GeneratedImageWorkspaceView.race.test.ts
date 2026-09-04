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

const i18n = createI18n({ legacy: false, locale: 'vi', messages: { vi: viMessages } })

const plan = {
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

describe('GeneratedImageWorkspaceView request races', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    sessionStorage.clear()
    vi.spyOn(projectApi, 'getProject').mockResolvedValue({
      id: 'proj-123',
      title: 'Test Project',
      description: '',
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
    vi.spyOn(scenePlanApi, 'getScenePlan').mockResolvedValue(plan)
    vi.spyOn(imageApi, 'fetchImageGenerationOptions').mockResolvedValue({
      providers: [
        { id: 'openai', display_name: 'OpenAI', models: [{ id: 'image-1', display_name: 'Image 1' }] },
      ],
    })
    vi.spyOn(mediaApi, 'listSceneMediaBindings').mockResolvedValue([
      { scene_key: 'scene-1', role: 'primary_visual' },
    ])
    vi.spyOn(mediaApi, 'listMediaAssets').mockResolvedValue({ assets: [] })
  })

  it('ignores a late response from an older request after a newer regeneration wins', async () => {
    const randomUUID = vi
      .fn()
      .mockReturnValueOnce('11111111-1111-4111-8111-111111111111')
      .mockReturnValueOnce('22222222-2222-4222-8222-222222222222')
    vi.stubGlobal('crypto', { randomUUID })

    let resolveOld!: (job: imageApi.SceneImageGenerationJobView) => void
    const oldPoll = new Promise<imageApi.SceneImageGenerationJobView>((resolve) => {
      resolveOld = resolve
    })
    vi.spyOn(imageApi, 'getSceneImageGeneration').mockReturnValue(oldPoll)
    vi.spyOn(imageApi, 'createSceneImageGeneration').mockImplementation(
      async (_projectID, _version, _sceneKey, input) => {
        if (input.request_id.startsWith('1111')) {
          return {
            id: input.request_id,
            state: 'queued',
            attempt: 0,
            max_attempts: 3,
            assigned_primary_visual: false,
            created_at: '2026-09-04T00:00:00Z',
            updated_at: '2026-09-04T00:00:00Z',
          }
        }
        return {
          id: input.request_id,
          state: 'succeeded',
          attempt: 1,
          max_attempts: 3,
          media_asset_id: 'asset-new',
          assigned_primary_visual: false,
          created_at: '2026-09-04T00:00:01Z',
          updated_at: '2026-09-04T00:00:02Z',
        }
      },
    )

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/projects/:id/images', name: 'generated-images', component: GeneratedImageWorkspaceView },
      ],
    })
    router.push('/projects/proj-123/images')
    await router.isReady()
    const wrapper = mount(GeneratedImageWorkspaceView, { global: { plugins: [router, i18n] } })
    await flushPromises()

    const button = wrapper.get('[data-testid="generate-image-scene-1"]')
    void button.trigger('click')
    await Promise.resolve()
    await Promise.resolve()

    ;(button.element as HTMLButtonElement).disabled = false
    await button.trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="generated-image-scene-1"]').attributes('src')).toContain('asset-new')

    resolveOld({
      id: '11111111-1111-4111-8111-111111111111',
      state: 'succeeded',
      attempt: 1,
      max_attempts: 3,
      media_asset_id: 'asset-old',
      assigned_primary_visual: false,
      created_at: '2026-09-04T00:00:00Z',
      updated_at: '2026-09-04T00:00:03Z',
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="generated-image-scene-1"]').attributes('src')).toContain('asset-new')
  })
})
