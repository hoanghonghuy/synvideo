import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  deleteMediaAsset,
  listMediaAssets,
  mediaAssetContentURL,
  uploadMediaAsset,
} from './api'

afterEach(() => {
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('media API', () => {
  it('resolves list requests against the configured API origin', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example')
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ assets: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await listMediaAssets('project-1')

    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.example/api/v1/projects/project-1/media-assets',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('resolves mutation requests against the configured API origin', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example')
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    await deleteMediaAsset('project-1', 'asset-9')

    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.example/api/v1/projects/project-1/media-assets/asset-9',
      expect.objectContaining({ method: 'DELETE', credentials: 'include' }),
    )
  })

  it('uses the configured API origin for non-XHR upload fallback', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example')
    vi.stubGlobal('XMLHttpRequest', undefined as unknown as typeof XMLHttpRequest)
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: 'asset-1',
          project_id: 'project-1',
          kind: 'image',
          origin: 'upload',
          mime_type: 'image/png',
          byte_size: 12,
          sha256: 'abc',
          created_at: '2026-09-09T00:00:00Z',
          updated_at: '2026-09-09T00:00:00Z',
        }),
        { status: 201, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    const file = new File(['pixels'], 'cover.png', { type: 'image/png' })
    await uploadMediaAsset('project-1', file)

    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.example/api/v1/projects/project-1/media-assets',
      expect.objectContaining({ method: 'POST', credentials: 'include' }),
    )
  })

  it('builds absolute browser content URLs from the configured API origin', () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example')

    expect(mediaAssetContentURL('project-1', 'asset-9')).toBe(
      'https://api.example/api/v1/projects/project-1/media-assets/asset-9/content',
    )
  })
})
