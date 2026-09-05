import { afterEach, describe, expect, it, vi } from 'vitest'

import { getCaptions, rebuildCaptions, updateCaptions, type CaptionView } from './api'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const view: CaptionView = {
  id: 'caption-1',
  project_id: 'project/one',
  scene_plan_version: 2,
  scene_key: 'scene intro',
  revision: 3,
  source_binding_id: 'binding-1',
  source_asset_id: 'asset-1',
  source_duration_ms: 4200,
  segments: [{ id: 'segment-1', text: 'Hello', start_ms: 0, end_ms: 4200 }],
  style: { alignment: 'center', position: 'bottom', size: 'medium', weight: 'normal' },
  created_at: '2026-09-05T00:00:00Z',
  state: 'CURRENT',
}

describe('caption API', () => {
  it('loads the exact project plan and scene identity', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(view), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await getCaptions('project/one', 2, 'scene intro')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project%2Fone/scene-plans/2/scenes/scene%20intro/captions',
      { headers: { Accept: 'application/json' } },
    )
  })

  it('sends optimistic revision and canonical millisecond timing', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...view, revision: 4 }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await updateCaptions('project-1', 2, 'scene-1', {
      expected_revision: 3,
      segments: [{ id: 'segment-1', text: 'Edited', start_ms: 100, end_ms: 4000 }],
      style: view.style,
    })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project-1/scene-plans/2/scenes/scene-1/captions',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          expected_revision: 3,
          segments: [{ id: 'segment-1', text: 'Edited', start_ms: 100, end_ms: 4000 }],
          style: view.style,
        }),
      }),
    )
  })

  it('makes rebuild explicit and revision guarded', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...view, revision: 4 }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await rebuildCaptions('project-1', 2, 'scene-1', 3)

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project-1/scene-plans/2/scenes/scene-1/captions/rebuild',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ expected_revision: 3 }) }),
    )
  })
})
