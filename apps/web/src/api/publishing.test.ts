import { afterEach, describe, expect, it, vi } from 'vitest'

import { createPublishAttempt, getPublishAttempt, listPublishingConnections } from './publishing'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('publishing API', () => {
  it('loads owner-scoped publishing connections', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await listPublishingConnections()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/publishing/connections',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('creates a publish attempt with exact request identity and encoded project identity', async () => {
    const payload = {
      connection_id: 'connection-1',
      render_artifact_id: 'artifact-1',
      request_id: 'request-1',
      title: 'Launch video',
      description: 'Description',
    }
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'attempt-1' }), { status: 201 }))
    vi.stubGlobal('fetch', fetchMock)

    await createPublishAttempt('project/one', payload)

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project%2Fone/publishing/attempts',
      expect.objectContaining({ method: 'POST', body: JSON.stringify(payload) }),
    )
  })

  it('loads an attempt with encoded project and attempt identities', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'attempt-1' }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await getPublishAttempt('project/one', 'attempt/two')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/projects/project%2Fone/publishing/attempts/attempt%2Ftwo',
      expect.objectContaining({ credentials: 'include' }),
    )
  })
})
