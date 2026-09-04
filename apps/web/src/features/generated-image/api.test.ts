import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  createSceneImageGeneration,
  fetchImageGenerationOptions,
  getSceneImageGeneration,
} from './api'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('generated image API', () => {
  it('loads only the owner-safe enabled image generation options endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          providers: [
            {
              id: 'openai',
              display_name: 'OpenAI',
              models: [{ id: 'image-1', display_name: 'Image 1' }],
            },
          ],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    const result = await fetchImageGenerationOptions()

    expect(result.providers[0]?.id).toBe('openai')
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/ai/image-generation-options', {
      headers: { Accept: 'application/json' },
    })
  })

  it('submits the edited prompt and stable request id to the exact scene', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: 'request-123',
          state: 'queued',
          attempt: 0,
          max_attempts: 3,
          assigned_primary_visual: false,
          created_at: '2026-09-04T00:00:00Z',
          updated_at: '2026-09-04T00:00:00Z',
        }),
        { status: 202, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    await createSceneImageGeneration('project/one', 4, 'scene intro', {
      request_id: 'request-123',
      provider_id: 'openai',
      model_id: 'image-1',
      prompt: 'Edited cinematic prompt',
      assign_primary_visual: false,
    })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project%2Fone/scene-plans/4/scenes/scene%20intro/image-generations',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          request_id: 'request-123',
          provider_id: 'openai',
          model_id: 'image-1',
          prompt: 'Edited cinematic prompt',
          assign_primary_visual: false,
        }),
      }),
    )
  })

  it('recovers the exact durable job instead of creating a new request', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: 'request-123',
          state: 'succeeded',
          attempt: 1,
          max_attempts: 3,
          media_asset_id: 'asset-9',
          assigned_primary_visual: false,
          created_at: '2026-09-04T00:00:00Z',
          updated_at: '2026-09-04T00:00:01Z',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    const job = await getSceneImageGeneration('project/one', 'request/123')

    expect(job.media_asset_id).toBe('asset-9')
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project%2Fone/image-generations/request%2F123',
      { headers: { Accept: 'application/json' } },
    )
  })

  it.each([
    ['ERR_IMAGE_PROVIDER_TIMEOUT', 'providerUnavailable'],
    ['ERR_IMAGE_PROVIDER_FAILED', 'providerUnavailable'],
    ['ERR_IMAGE_ASSIGNMENT_FAILED', 'assignmentFailed'],
    ['ERR_IMAGE_INTERNAL_SECRET_DETAIL', 'requestFailed'],
  ])('maps durable terminal error %s to creator-safe state %s', async (backendCode, safeCode) => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: 'request-123',
          state: 'failed',
          attempt: 3,
          max_attempts: 3,
          error_code: backendCode,
          media_asset_id: backendCode === 'ERR_IMAGE_ASSIGNMENT_FAILED' ? 'asset-9' : undefined,
          assigned_primary_visual: false,
          created_at: '2026-09-04T00:00:00Z',
          updated_at: '2026-09-04T00:00:03Z',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    const job = await getSceneImageGeneration('project-1', 'request-123')

    expect(job.error_code).toBe(safeCode)
    expect(job.error_code).not.toBe(backendCode)
    if (backendCode === 'ERR_IMAGE_ASSIGNMENT_FAILED') {
      expect(job.media_asset_id).toBe('asset-9')
    }
  })
})
