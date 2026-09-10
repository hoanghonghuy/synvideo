import { afterEach, describe, expect, it, vi } from 'vitest'

import { clearAccessToken, getAccessToken, setAccessToken } from '@/auth/session'
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
    vi.unstubAllGlobals()
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

  it('clears an expired in-memory credential and routes 401 to bounded re-auth', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    setAccessToken('expired-token')
    const assign = vi.fn()
    vi.stubGlobal('location', {
      origin: 'https://app.example',
      pathname: '/projects/project-1',
      search: '?tab=timeline',
      hash: '#clip-3',
      assign,
    })
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{}', { status: 401 }))

    const response = await apiFetch('/api/v1/projects/project-1')

    expect(response.status).toBe(401)
    expect(getAccessToken()).toBeNull()
    expect(assign).toHaveBeenCalledOnce()
    expect(assign.mock.calls[0]?.[0]).toBe('/sign-in?reason=session-expired&returnTo=%2Fprojects%2Fproject-1%3Ftab%3Dtimeline%23clip-3')
  })

  it('does not reinterpret ownership 403 as an authentication failure', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    setAccessToken('valid-token')
    const assign = vi.fn()
    vi.stubGlobal('location', {
      origin: 'https://app.example',
      pathname: '/projects/project-1',
      search: '',
      hash: '',
      assign,
    })
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{}', { status: 403 }))

    const response = await apiFetch('/api/v1/projects/project-1')

    expect(response.status).toBe(403)
    expect(getAccessToken()).toBe('valid-token')
    expect(assign).not.toHaveBeenCalled()
  })

  it('does not create a redirect loop while already on an auth route', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    setAccessToken('expired-token')
    const assign = vi.fn()
    vi.stubGlobal('location', {
      origin: 'https://app.example',
      pathname: '/auth/callback',
      search: '?code=x&state=y',
      hash: '',
      assign,
    })
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{}', { status: 401 }))

    await apiFetch('/api/v1/projects')

    expect(getAccessToken()).toBeNull()
    expect(assign).not.toHaveBeenCalled()
  })
})
