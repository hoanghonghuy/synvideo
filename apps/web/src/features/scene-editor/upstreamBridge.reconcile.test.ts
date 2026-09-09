import { beforeEach, describe, expect, it, vi } from 'vitest'

import { buildReconcileCandidate, scenePlanSceneKeys } from './upstreamBridge'

const getScenePlan = vi.fn()
const listSceneMediaBindings = vi.fn()
const listSceneNarrations = vi.fn()
const getAudioMix = vi.fn()
const getCaptions = vi.fn()

vi.mock('@/features/scene-plan/api', () => ({
  getScenePlan: (...args: unknown[]) => getScenePlan(...args),
  listScenePlans: vi.fn(),
}))

vi.mock('@/features/media/api', () => ({
  listSceneMediaBindings: (...args: unknown[]) => listSceneMediaBindings(...args),
}))

vi.mock('@/features/scene-narration/api', () => ({
  listSceneNarrations: (...args: unknown[]) => listSceneNarrations(...args),
}))

vi.mock('@/features/audio-mix/api', () => ({
  getAudioMix: (...args: unknown[]) => getAudioMix(...args),
}))

vi.mock('@/features/captions/api', () => ({
  getCaptions: (...args: unknown[]) => getCaptions(...args),
}))

describe('buildReconcileCandidate', () => {
  beforeEach(() => {
    getScenePlan.mockReset()
    listSceneMediaBindings.mockReset()
    listSceneNarrations.mockReset()
    getAudioMix.mockReset()
    getCaptions.mockReset()
    listSceneMediaBindings.mockResolvedValue([])
    listSceneNarrations.mockResolvedValue([])
    getAudioMix.mockRejectedValue(new Error('missing'))
  })

  it('uses authoritative scene plan keys instead of composition keys', async () => {
    getScenePlan.mockResolvedValue({
      version: 3,
      scenes: [
        { key: 'intro', script_section_key: 'intro', narration: '', visual_instruction: '', planned_source_type: 'stock', expected_duration_seconds: 5 },
        { key: 'main', script_section_key: 'section-1', narration: '', visual_instruction: '', planned_source_type: 'stock', expected_duration_seconds: 8 },
      ],
    })

    const candidate = await buildReconcileCandidate('project-1', 3)

    expect(scenePlanSceneKeys({ scenes: [{ key: 'intro' }, { key: 'main' }] })).toEqual(['intro', 'main'])
    expect(candidate.scene_plan_version).toBe(3)
    expect(candidate.scenes.map((scene) => scene.scene_key)).toEqual(['intro', 'main'])
    expect(listSceneMediaBindings).toHaveBeenCalledWith('project-1', 3)
    expect(listSceneNarrations).toHaveBeenCalledWith('project-1', 3)
    expect(getScenePlan).toHaveBeenCalledWith('project-1', 3)
  })

  it('does not query removed composition-only keys when plan shrinks', async () => {
    getScenePlan.mockResolvedValue({
      version: 4,
      scenes: [
        { key: 'hook', script_section_key: 'intro', narration: '', visual_instruction: '', planned_source_type: 'stock', expected_duration_seconds: 5 },
      ],
    })

    const candidate = await buildReconcileCandidate('project-1', 4)

    expect(candidate.scenes).toEqual([{ scene_key: 'hook', visual: undefined, narration: undefined, caption: undefined }])
    expect(getCaptions).not.toHaveBeenCalled()
  })
})
