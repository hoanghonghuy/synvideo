import { afterEach, describe, expect, it, vi } from 'vitest'

import { createAudioMix, getAudioMix, rebindAudioMixNarration, updateAudioMix, type AudioMixConfig, type AudioMixView } from './api'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const config: AudioMixConfig = {
  music_trim_start_ms: 100,
  start_offset_ms: 250,
  loop_policy: 'LOOP_TO_TARGET',
  music_gain_db: -12,
  narration_gain_db: 0,
  ducking: { enabled: true, reduction_db: 9, attack_ms: 120, release_ms: 350 },
}

const view: AudioMixView = {
  id: 'mix-1',
  project_id: 'project/one',
  revision: 3,
  scene_plan_version: 4,
  music_asset_id: 'music-1',
  music_duration_ms: 90000,
  narration_lineage_id: 'lineage-1',
  narration_duration_ms: 60000,
  config,
  created_at: '2026-09-06T00:00:00Z',
  updated_at: '2026-09-06T00:00:00Z',
  state: 'CURRENT',
}

describe('audio mix API', () => {
  it('escapes the project identity when loading', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(view), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await getAudioMix('project/one')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project%2Fone/audio-mix',
      expect.objectContaining({ headers: expect.objectContaining({ Accept: 'application/json' }) }),
    )
  })

  it('creates a mix from a durable music asset and explicit config', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(view), { status: 201 }))
    vi.stubGlobal('fetch', fetchMock)

    await createAudioMix('project-1', { music_asset_id: 'music-1', config })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project-1/audio-mix',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ music_asset_id: 'music-1', config }) }),
    )
  })

  it('guards updates with the exact expected revision', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...view, revision: 4 }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await updateAudioMix('project-1', { expected_revision: 3, music_asset_id: 'music-1', config })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project-1/audio-mix',
      expect.objectContaining({ method: 'PUT', body: JSON.stringify({ expected_revision: 3, music_asset_id: 'music-1', config }) }),
    )
  })

  it('makes narration rebinding explicit and revision guarded', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...view, revision: 4 }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await rebindAudioMixNarration('project-1', 3)

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project-1/audio-mix/rebind-narration',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ expected_revision: 3 }) }),
    )
  })
})
