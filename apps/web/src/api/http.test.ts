import { afterEach, describe, expect, it, vi } from 'vitest'

import { clearAccessToken, setAccessToken } from '@/auth/session'
import { apiFetch, apiUrl } from './http'

describe('apiUrl', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('keeps relative paths for same-origin development', () => {
    vi.stubEnv('VITE_API_BASE_URL', '')
    expect(apiUrl('/api/v1/healthz')).toBe('/api/v1/healthz')
  })

  it('prefixes configured production API origin', () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example.onrender.com')
    expect(apiUrl('/api/v1/readyz')).toBe('https://api.example.onrender.com/api/v1/readyz')
  })
})

describe('apiFetch', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
    clearAccessToken()
    vi.restoreAllMocks()
  })

  it('attaches bearer authorization through the shared request layer', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example.onrender.com')
    setAccessToken('memory-only-token')

    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{}', { status: 200 }))

    await apiFetch('/api/v1/projects')

    expect(fetchMock).toHaveBeenCalledOnce()
    const call = fetchMock.mock.calls[0]
    if (!call) {
      throw new Error('expected fetch to be called')
    }
    const init = call[1] as RequestInit
    const headers = new Headers(init.headers)
    expect(headers.get('Authorization')).toBe('Bearer memory-only-token')
    expect(localStorage.getItem('access_token')).toBeNull()
    expect(sessionStorage.getItem('access_token')).toBeNull()
  })
})
