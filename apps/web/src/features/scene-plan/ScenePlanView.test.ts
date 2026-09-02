import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { i18n } from '@/locales'
import ScenePlanView from './ScenePlanView.vue'

const fetchMock = vi.fn()
const projectId = '11111111-1111-4111-8111-111111111111'
const jobId = '22222222-2222-4222-8222-222222222222'

const project = {
  id: projectId,
  title: 'Video phan canh',
  description: 'Mo ta du an',
  content_format: 'long',
  aspect_ratio: '16:9',
  target_duration_seconds: 600,
  locale: 'vi',
  status: 'active',
  created_at: '2026-08-31T08:00:00Z',
  updated_at: '2026-08-31T08:30:00Z',
}

const approvedScript = {
  project_id: projectId,
  version: 2,
  revision: 1,
  status: 'approved',
  source_proposal_version: 1,
  content_locale: 'vi',
  sections: [
    { key: 'sec_intro', heading: 'Mở đầu', body: 'Chào mừng các bạn đến với video ngày hôm nay.' },
    { key: 'sec_body', heading: 'Thân bài', body: 'Chúng ta sẽ cùng tìm hiểu về hệ thống SynVideo.' },
  ],
  estimated_duration_seconds: 60,
  notes: 'Ghi chú',
  created_at: '2026-08-31T08:30:00Z',
  updated_at: '2026-08-31T08:35:00Z',
  approved_at: '2026-08-31T08:36:00Z',
}

const draftPlan = {
  project_id: projectId,
  version: 1,
  revision: 2,
  status: 'draft',
  source_script_version: 2,
  source_proposal_version: 1,
  content_locale: 'vi',
  scenes: [
    {
      key: 'scene-intro-1',
      script_section_key: 'sec_intro',
      narration: 'Chào mừng các bạn đến với video ngày hôm nay.',
      visual_instruction: 'Toàn cảnh trường quay hiện đại.',
      planned_source_type: 'stock',
      expected_duration_seconds: 15,
      caption_intent: 'Chào mừng',
      transition_notes: 'Fade in',
    },
    {
      key: 'scene-body-1',
      script_section_key: 'sec_body',
      narration: 'Chúng ta sẽ cùng tìm hiểu về hệ thống SynVideo.',
      visual_instruction: 'Cận cảnh giao diện phần mềm.',
      planned_source_type: 'creator_media',
      expected_duration_seconds: 45,
      caption_intent: 'Giới thiệu',
      transition_notes: 'Cut',
    },
  ],
  created_at: '2026-08-31T09:00:00Z',
  updated_at: '2026-08-31T09:30:00Z',
  approved_at: null,
}

const approvedPlan = {
  ...draftPlan,
  version: 2,
  revision: 1,
  status: 'approved',
  approved_at: '2026-08-31T09:35:00Z',
}

const providers = {
  providers: [
    {
      id: 'provider-openai',
      display_name: 'OpenAI',
      models: [{ id: 'gpt-4o', display_name: 'GPT-4o' }],
    },
  ],
}

type Handler = (url: string, init?: RequestInit) => unknown
let handlers: { method?: string; path: string; handler: Handler }[] = []

beforeEach(() => {
  handlers = []
  window.sessionStorage.clear()
  fetchMock.mockReset()
  fetchMock.mockImplementation((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    const match = [...handlers].reverse().find((item) => {
      return (!item.method || item.method === method) && item.path === url
    })
    if (match) return Promise.resolve(match.handler(url, init))
    if (url === '/api/v1/ai/text-generation-options') return Promise.resolve(jsonResponse({ providers: [] }))
    return Promise.resolve(jsonResponse({ error: { code: 'not_found' } }, 404))
  })
  vi.stubGlobal('fetch', fetchMock)
  i18n.global.locale.value = 'vi'
})

describe('ScenePlanView', () => {
  it('1. renders a true empty readiness state when there is no scene plan', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [])
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
    route('GET', '/api/v1/ai/text-generation-options', providers)

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="scene-plan-empty-state"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="generate-scene-plan-btn"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).toContain('Chưa có Scene Plan')
  })

  it('1b. renders a retryable load error instead of a false empty state when initial list fails', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, { error: { code: 'request_failed' } }, 503)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="scene-plan-empty-state"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="retry-scene-plan-list"]').exists()).toBe(true)
  })

  it('2. opens active draft automatically on initial load when present', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedPlan, draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
    route('GET', '/api/v1/ai/text-generation-options', providers)

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="version-1"]').classes()).toContain('active')
    expect(wrapper.text()).toContain('Phiên bản 1')
    expect(wrapper.text()).toContain('Bản sửa 2')
  })

  it('3. guards against losing dirty edits on version switch and allows discard', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [approvedPlan, draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
    route('GET', '/api/v1/ai/text-generation-options', providers)

    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('[name="scene_visual_0"]').setValue('Hình ảnh đã chỉnh sửa mới')
    expect(wrapper.find('[data-testid="dirty-state"]').exists()).toBe(true)

    // Attempt to switch to version 2
    await wrapper.find('[data-testid="version-2"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="dirty-switch-warning"]').exists()).toBe(true)
    expect(fetchMock.mock.calls.filter(([url, init]) => url.endsWith('/scene-plans/2') && (!init || !init.method)).length).toBe(0)

    // Discard and switch
    await wrapper.find('[data-testid="confirm-discard-switch"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="version-2"]').classes()).toContain('active')
    expect(wrapper.text()).toContain('Phiên bản 2')
  })

  it('4. keeps approved and superseded versions read-only', async () => {
    const supersededPlan = { ...approvedPlan, version: 3, status: 'superseded' as const, approved_at: null }
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [supersededPlan, approvedPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/3`, supersededPlan)
    route('GET', `/api/v1/projects/${projectId}/scene-plans/2`, approvedPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="version-3"]').text()).toContain('Đã thay thế')
    expect((wrapper.find('[name="scene_visual_0"]').element as HTMLTextAreaElement).disabled).toBe(true)
    expect(wrapper.find('[data-testid="save-scene-plan-btn"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="split-scene-0"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="merge-scene-0"]').exists()).toBe(false)
  })

  it('5. shows stale source warning when higher approved script version exists', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    // Highest approved script version is 3, while draftPlan has source_script_version = 2
    route('GET', `/api/v1/projects/${projectId}/scripts`, [{ ...approvedScript, version: 3 }])
    route('GET', '/api/v1/ai/text-generation-options', providers)

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="stale-source-warning"]').exists()).toBe(true)
  })

  it('6. edits planning fields and saves clean optimistic revision', async () => {
    const updatedPlan = {
      ...draftPlan,
      revision: 3,
      scenes: [
        {
          ...draftPlan.scenes[0],
          visual_instruction: 'Visual mới cập nhật',
          planned_source_type: 'generated_image',
          expected_duration_seconds: 20,
          caption_intent: 'Caption mới',
          transition_notes: 'Zoom in',
        },
        draftPlan.scenes[1],
      ],
    }

    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
    route('PUT', `/api/v1/projects/${projectId}/scene-plans/1`, updatedPlan)

    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('[name="scene_visual_0"]').setValue('Visual mới cập nhật')
    await wrapper.find('[name="scene_source_type_0"]').setValue('generated_image')
    await wrapper.find('[name="scene_duration_0"]').setValue('20')
    await wrapper.find('[name="scene_caption_0"]').setValue('Caption mới')
    await wrapper.find('[name="scene_transition_0"]').setValue('Zoom in')

    await wrapper.find('form.scene-plan-form').trigger('submit')
    await flushPromises()

    const putCall = fetchMock.mock.calls.find(
      ([url, init]) => url === `/api/v1/projects/${projectId}/scene-plans/1` && init?.method === 'PUT',
    )
    expect(putCall).toBeDefined()
    const body = JSON.parse(String(putCall?.[1]?.body))
    expect(body.revision).toBe(2)
    expect(body.scenes[0].visual_instruction).toBe('Visual mới cập nhật')
    expect(body.scenes[0].planned_source_type).toBe('generated_image')
    expect(body.scenes[0].expected_duration_seconds).toBe(20)

    expect(wrapper.text()).toContain('Đã lưu Scene Plan.')
    expect(wrapper.text()).toContain('Bản sửa 3')
  })

  it('7. preserves local edits on stale save conflict (409 STALE_REVISION) and supports reload', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
    let planLoads = 0
    routeFn('GET', `/api/v1/projects/${projectId}/scene-plans/1`, () => {
      planLoads++
      return jsonResponse(
        planLoads === 1
          ? draftPlan
          : {
              ...draftPlan,
              revision: 4,
              scenes: [{ ...draftPlan.scenes[0], visual_instruction: 'Bản từ máy chủ' }, draftPlan.scenes[1]],
            },
      )
    })
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
    route('PUT', `/api/v1/projects/${projectId}/scene-plans/1`, { error: { code: 'STALE_REVISION' } }, 409)

    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('[name="scene_visual_0"]').setValue('Bản chỉnh sửa cục bộ không được mất')
    await wrapper.find('form.scene-plan-form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="dirty-state"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Scene Plan trên máy chủ đã thay đổi.')
    expect((wrapper.find('[name="scene_visual_0"]').element as HTMLTextAreaElement).value).toBe(
      'Bản chỉnh sửa cục bộ không được mất',
    )

    // Trigger reload stale
    await wrapper.find('[data-testid="reload-stale-scene-plan"]').trigger('click')
    expect(wrapper.find('[data-testid="confirm-stale-reload"]').exists()).toBe(true)
    await wrapper.find('[data-testid="confirm-reload-stale-scene-plan"]').trigger('click')
    await flushPromises()

    expect((wrapper.find('[name="scene_visual_0"]').element as HTMLTextAreaElement).value).toBe('Bản từ máy chủ')
  })

  it('8. splits a scene into two preserving exact narration and section key', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])

    const wrapper = await mountView()
    await flushPromises()

    const originalNarration = draftPlan.scenes[0]!.narration
    // Open split dialog for scene 0
    await wrapper.find('[data-testid="split-scene-0"]').trigger('click')
    expect(wrapper.find('[data-testid="split-modal"]').exists()).toBe(true)

    // Split index at character 20
    const splitIndex = 20
    await wrapper.find('[data-testid="split-index-input"]').setValue(String(splitIndex))
    await wrapper.find('[data-testid="confirm-split-btn"]').trigger('click')
    await flushPromises()

    // Expect scenes count increased to 3
    const scenes = wrapper.findAll('.scene-card')
    expect(scenes.length).toBe(3)

    // Scene 0 and Scene 1 must share the same script_section_key
    const scene0Narration = wrapper.find('[data-testid="scene-narration-0"]').text()
    const scene1Narration = wrapper.find('[data-testid="scene-narration-1"]').text()

    expect(scene0Narration + scene1Narration).toBe(originalNarration)
    expect(wrapper.find('[data-testid="scene-section-0"]').text()).toContain('sec_intro')
    expect(wrapper.find('[data-testid="scene-section-1"]').text()).toContain('sec_intro')
    expect(wrapper.find('[data-testid="dirty-state"]').exists()).toBe(true)
  })

  it('9. merges adjacent scenes only when they belong to the same section and preserves concatenation', async () => {
    // Setup plan with 2 scenes in section 1 and 1 scene in section 2
    const multiScenePlan = {
      ...draftPlan,
      scenes: [
        {
          key: 'scene-intro-1',
          script_section_key: 'sec_intro',
          narration: 'Chào mừng các bạn ',
          visual_instruction: 'Visual 1',
          planned_source_type: 'stock',
          expected_duration_seconds: 7,
        },
        {
          key: 'scene-intro-2',
          script_section_key: 'sec_intro',
          narration: 'đến với video ngày hôm nay.',
          visual_instruction: 'Visual 2',
          planned_source_type: 'stock',
          expected_duration_seconds: 8,
        },
        {
          key: 'scene-body-1',
          script_section_key: 'sec_body',
          narration: 'Chúng ta sẽ cùng tìm hiểu về hệ thống SynVideo.',
          visual_instruction: 'Visual 3',
          planned_source_type: 'creator_media',
          expected_duration_seconds: 45,
        },
      ],
    }

    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [multiScenePlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, multiScenePlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])

    const wrapper = await mountView()
    await flushPromises()

    // Scene 0 (sec_intro) and Scene 1 (sec_intro) can be merged: merge button should exist on scene 0
    expect(wrapper.find('[data-testid="merge-scene-0"]').exists()).toBe(true)

    // Scene 1 (sec_intro) and Scene 2 (sec_body) cannot be merged: merge button must not exist or be disabled
    expect(wrapper.find('[data-testid="merge-scene-1"]').exists()).toBe(false)

    // Trigger merge on scene 0
    await wrapper.find('[data-testid="merge-scene-0"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.scene-card').length).toBe(2)
    expect(wrapper.find('[data-testid="scene-narration-0"]').text()).toBe(
      'Chào mừng các bạn đến với video ngày hôm nay.',
    )
    expect(wrapper.find('[data-testid="dirty-state"]').exists()).toBe(true)
  })

  it('10. does not expose freeform narration rewrite inputs', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])

    const wrapper = await mountView()
    await flushPromises()

    // Confirm that narration is not an editable input/textarea
    expect(wrapper.find('textarea[name="narration_0"]').exists()).toBe(false)
    expect(wrapper.find('input[name="narration_0"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="scene-narration-0"]').exists()).toBe(true)
  })

  it('11. blocks generate and approve when draft is dirty', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
    route('GET', '/api/v1/ai/text-generation-options', providers)

    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('[name="scene_visual_0"]').setValue('Chỉnh sửa chưa lưu')
    expect(wrapper.find('[data-testid="dirty-state"]').exists()).toBe(true)

    expect(wrapper.find('[data-testid="generate-scene-plan-btn"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="approve-scene-plan-btn"]').attributes('disabled')).toBeDefined()
  })

  it('12. shows guidance linking to AI settings when no provider models are configured', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
    route('GET', '/api/v1/ai/text-generation-options', { providers: [] })

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="no-scene-plan-providers-notice"]').exists()).toBe(true)
    expect(wrapper.find('a[href="/settings/ai-providers"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="generate-scene-plan-btn"]').exists()).toBe(false)
  })

  it('13. generates unique UUID request ID for each explicit generation call', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      route('GET', `/api/v1/projects/${projectId}`, project)
      route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
      route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
      route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
      route('GET', '/api/v1/ai/text-generation-options', providers)
      route('POST', `/api/v1/projects/${projectId}/scene-plan-generations`, job('queued'), 202)
      route('GET', `/api/v1/projects/${projectId}/scene-plan-generations/${jobId}`, job('running'))

      const wrapper = await mountView()
      await flushPromises()

      await wrapper.find('[data-testid="generate-scene-plan-btn"]').trigger('click')
      await flushPromises()

      const postCall = fetchMock.mock.calls.find(
        ([url, init]) =>
          url === `/api/v1/projects/${projectId}/scene-plan-generations` && init?.method === 'POST',
      )
      expect(postCall).toBeDefined()
      const body = JSON.parse(String(postCall?.[1]?.body))
      expect(body.request_id).toMatch(/^[0-9a-f-]{36}$/)
    } finally {
      vi.useRealTimers()
    }
  })

  it('14. preserves currently displayed scene plan during queued/running generation', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      route('GET', `/api/v1/projects/${projectId}`, project)
      route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
      route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
      route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
      route('GET', '/api/v1/ai/text-generation-options', providers)
      route('POST', `/api/v1/projects/${projectId}/scene-plan-generations`, job('queued'), 202)
      route('GET', `/api/v1/projects/${projectId}/scene-plan-generations/${jobId}`, job('running'))

      const wrapper = await mountView()
      await flushPromises()

      await wrapper.find('[data-testid="generate-scene-plan-btn"]').trigger('click')
      await flushPromises()

      expect(wrapper.find('[data-testid="job-state-queued"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="scene-narration-0"]').exists()).toBe(true)
      expect(wrapper.text()).toContain('Phiên bản 1')

      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()

      expect(wrapper.find('[data-testid="job-state-running"]').exists()).toBe(true)
      // The displayed plan must still be visible and loaded
      expect(wrapper.find('[data-testid="scene-narration-0"]').exists()).toBe(true)
      expect(wrapper.text()).toContain('Phiên bản 1')
    } finally {
      vi.useRealTimers()
    }
  })

  it('15. resumes active durable generation job from sessionStorage upon refresh', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      window.sessionStorage.setItem(`scene_plan_job_${projectId}`, jobId)
      route('GET', `/api/v1/projects/${projectId}`, project)
      route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
      route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
      route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
      route('GET', '/api/v1/ai/text-generation-options', providers)
      route('GET', `/api/v1/projects/${projectId}/scene-plan-generations/${jobId}`, job('running'))

      const wrapper = await mountView()
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()

      expect(wrapper.find('[data-testid="job-state-running"]').exists()).toBe(true)
      expect(
        fetchMock.mock.calls.some(
          ([url]) => url === `/api/v1/projects/${projectId}/scene-plan-generations/${jobId}`,
        ),
      ).toBe(true)
    } finally {
      vi.useRealTimers()
    }
  })

  it('16. retries same job on transient poll error without POSTing replacement', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      window.sessionStorage.setItem(`scene_plan_job_${projectId}`, jobId)
      route('GET', `/api/v1/projects/${projectId}`, project)
      route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
      route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
      route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
      route('GET', '/api/v1/ai/text-generation-options', providers)

      let polls = 0
      routeFn('GET', `/api/v1/projects/${projectId}/scene-plan-generations/${jobId}`, () => {
        polls++
        return polls === 1 ? jsonResponse({ error: { code: 'request_failed' } }, 503) : jsonResponse(job('running'))
      })

      const wrapper = await mountView()
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()

      expect(wrapper.find('[data-testid="job-error-banner"]').exists()).toBe(true)

      // Next poll recovers
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()

      expect(wrapper.find('[data-testid="job-state-running"]').exists()).toBe(true)
      expect(
        fetchMock.mock.calls.filter(
          ([url, init]) =>
            url === `/api/v1/projects/${projectId}/scene-plan-generations` && init?.method === 'POST',
        ).length,
      ).toBe(0)
    } finally {
      vi.useRealTimers()
    }
  })

  it('17. opens exact returned scene plan version on job success', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      route('GET', `/api/v1/projects/${projectId}`, project)
      let listCalls = 0
      routeFn('GET', `/api/v1/projects/${projectId}/scene-plans`, () => {
        listCalls++
        return jsonResponse(listCalls === 1 ? [] : [{ ...draftPlan, version: 5 }])
      })
      route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
      route('GET', '/api/v1/ai/text-generation-options', providers)
      route('POST', `/api/v1/projects/${projectId}/scene-plan-generations`, job('queued'), 202)
      route('GET', `/api/v1/projects/${projectId}/scene-plan-generations/${jobId}`, job('succeeded', 5))
      route('GET', `/api/v1/projects/${projectId}/scene-plans/5`, { ...draftPlan, version: 5 })

      const wrapper = await mountView()
      await flushPromises()

      await wrapper.find('[data-testid="generate-scene-plan-btn"]').trigger('click')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      await flushPromises()

      expect(wrapper.text()).toContain('Phiên bản 5')
      expect(fetchMock.mock.calls.some(([url]) => url === `/api/v1/projects/${projectId}/scene-plans/5`)).toBe(
        true,
      )
    } finally {
      vi.useRealTimers()
    }
  })

  it('18. retries loading succeeded version without regenerating if version fetch transiently fails', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      window.sessionStorage.setItem(`scene_plan_job_${projectId}`, jobId)
      route('GET', `/api/v1/projects/${projectId}`, project)
      route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
      route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
      route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
      route('GET', '/api/v1/ai/text-generation-options', providers)
      route('GET', `/api/v1/projects/${projectId}/scene-plan-generations/${jobId}`, job('succeeded', 8))

      let versionLoads = 0
      routeFn('GET', `/api/v1/projects/${projectId}/scene-plans/8`, () => {
        versionLoads++
        return versionLoads === 1
          ? jsonResponse({ error: { code: 'request_failed' } }, 503)
          : jsonResponse({ ...draftPlan, version: 8 })
      })

      const wrapper = await mountView()
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()

      expect(wrapper.find('[data-testid="retry-load-generated-btn"]').exists()).toBe(true)

      // Clicking retry loads the version, does not call POST
      await wrapper.find('[data-testid="retry-load-generated-btn"]').trigger('click')
      await flushPromises()

      expect(wrapper.text()).toContain('Phiên bản 8')
      expect(
        fetchMock.mock.calls.filter(
          ([url, init]) =>
            url === `/api/v1/projects/${projectId}/scene-plan-generations` && init?.method === 'POST',
        ).length,
      ).toBe(0)
    } finally {
      vi.useRealTimers()
    }
  })

  it('19. uses a fresh request ID on terminal retry after failed generation', async () => {
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])
    route('GET', '/api/v1/ai/text-generation-options', providers)

    let postCount = 0
    const requestIds: string[] = []
    routeFn('POST', `/api/v1/projects/${projectId}/scene-plan-generations`, (_url, init) => {
      postCount++
      const body = JSON.parse(String(init?.body))
      requestIds.push(body.request_id)
      return postCount === 1
        ? jsonResponse({ error: { code: 'GENERATION_PROVIDER_FAILED' } }, 500)
        : jsonResponse(job('queued'), 202)
    })

    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('[data-testid="generate-scene-plan-btn"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="retry-generation-btn"]').exists()).toBe(true)

    await wrapper.find('[data-testid="retry-generation-btn"]').trigger('click')
    await flushPromises()

    expect(requestIds.length).toBe(2)
    expect(requestIds[0]).not.toBe(requestIds[1])
  })

  it('20. approves a clean draft version atomically and updates status', async () => {
    const approvedDraft = { ...draftPlan, status: 'approved' as const, approved_at: '2026-08-31T10:00:00Z' }
    route('GET', `/api/v1/projects/${projectId}`, project)
    route('GET', `/api/v1/projects/${projectId}/scene-plans`, [draftPlan])
    route('GET', `/api/v1/projects/${projectId}/scene-plans/1`, draftPlan)
    route('GET', `/api/v1/projects/${projectId}/scripts`, [approvedScript])

    let listCalls = 0
    routeFn('GET', `/api/v1/projects/${projectId}/scene-plans`, () => {
      listCalls++
      return jsonResponse(listCalls === 1 ? [draftPlan] : [approvedDraft])
    })
    route('POST', `/api/v1/projects/${projectId}/scene-plans/1/approve`, approvedDraft)

    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('[data-testid="approve-scene-plan-btn"]').trigger('click')
    expect(wrapper.find('[data-testid="confirm-approve-modal"]').exists()).toBe(true)
    await wrapper.find('[data-testid="confirm-approve-btn"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Đã duyệt')
    expect((wrapper.find('[name="scene_visual_0"]').element as HTMLTextAreaElement).disabled).toBe(true)
  })
})

async function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/projects/:id/scene-plan', component: ScenePlanView },
      { path: '/settings/ai-providers', component: { template: '<div>Provider Settings</div>' } },
      { path: '/projects/:id/script', component: { template: '<div>Script View</div>' } },
    ],
  })
  router.push(`/projects/${projectId}/scene-plan`)
  await router.isReady()

  return mount(ScenePlanView, {
    global: {
      plugins: [i18n, router],
    },
  })
}

function route(method: string, path: string, body: unknown, status = 200) {
  handlers.push({ method, path, handler: () => jsonResponse(body, status) })
}

function routeFn(method: string, path: string, handler: Handler) {
  handlers.push({ method, path, handler })
}

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  }
}

function job(state: 'queued' | 'running' | 'succeeded' | 'failed', scenePlanVersion: number | null = null) {
  return {
    id: jobId,
    state,
    attempt: 1,
    max_attempts: 3,
    error_code: state === 'failed' ? 'GENERATION_FAILED' : null,
    scene_plan_version: scenePlanVersion,
    created_at: '2026-08-31T09:30:00Z',
    updated_at: '2026-08-31T09:31:00Z',
  }
}
