import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { clearAccessToken, getAccessToken, setAccessToken } from './session'
import { resetOidcDiscoveryCache } from './discovery'

describe('auth session storage boundary', () => {
  beforeEach(() => {
    clearAccessToken()
    sessionStorage.clear()
    localStorage.clear()
  })

  afterEach(() => {
    clearAccessToken()
    sessionStorage.clear()
    localStorage.clear()
  })

  it('keeps access tokens in memory only', () => {
    setAccessToken('in-memory-token')
    expect(getAccessToken()).toBe('in-memory-token')
    expect(localStorage.getItem('access_token')).toBeNull()
    expect(sessionStorage.getItem('access_token')).toBeNull()
    expect(localStorage.getItem('synvideo_access_token')).toBeNull()
    expect(sessionStorage.getItem('synvideo_access_token')).toBeNull()
  })
})

describe('loadOidcConfig', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('returns null when issuer or client id is missing', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', '')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', '')
    const { loadOidcConfig } = await import('./config')
    expect(loadOidcConfig()).toBeNull()
  })

  it('normalizes issuer and applies default scopes', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example/')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    vi.stubEnv('VITE_OIDC_AUDIENCE', 'synvideo-api')
    vi.stubEnv('VITE_OIDC_REDIRECT_URI', 'https://app.example/auth/callback')
    const { loadOidcConfig } = await import('./config')
    expect(loadOidcConfig()).toEqual({
      issuer: 'https://issuer.example',
      clientId: 'web-client',
      audience: 'synvideo-api',
      scopes: 'openid profile email',
      redirectUri: 'https://app.example/auth/callback',
    })
  })
})

describe('oidc discovery-backed flow', () => {
  afterEach(() => {
    resetOidcDiscoveryCache()
    vi.unstubAllEnvs()
    vi.restoreAllMocks()
    sessionStorage.clear()
    clearAccessToken()
  })

  it('redirects to discovered authorization endpoint instead of hardcoded /authorize', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    vi.stubEnv('VITE_OIDC_REDIRECT_URI', 'https://app.example/auth/callback')

    const assign = vi.fn()
    vi.stubGlobal('location', { assign })

    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      issuer: 'https://issuer.example',
      authorization_endpoint: 'https://issuer.example/connect/authorize',
      token_endpoint: 'https://issuer.example/connect/token',
    }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const { beginSignIn } = await import('./oidc')
    beginSignIn('/projects')

    await vi.waitFor(() => {
      expect(assign).toHaveBeenCalled()
    })

    const redirectTarget = assign.mock.calls[0]?.[0]
    if (typeof redirectTarget !== 'string') {
      throw new Error('expected authorization redirect URL')
    }
    const redirectURL = new URL(redirectTarget)
    expect(redirectURL.origin + redirectURL.pathname).toBe('https://issuer.example/connect/authorize')
    expect(redirectURL.searchParams.get('code_challenge_method')).toBe('S256')
  })

  it('exchanges authorization code at discovered token endpoint', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    vi.stubEnv('VITE_OIDC_REDIRECT_URI', 'https://app.example/auth/callback')

    sessionStorage.setItem('synvideo_oidc_flow_state', JSON.stringify({
      codeVerifier: 'verifier-123',
      state: 'state-123',
      returnTo: '/projects',
    }))

    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({
        issuer: 'https://issuer.example',
        authorization_endpoint: 'https://issuer.example/connect/authorize',
        token_endpoint: 'https://issuer.example/connect/token',
      }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'memory-only-token' }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const { completeSignInFromCallback } = await import('./oidc')
    const returnTo = await completeSignInFromCallback('?code=auth-code&state=state-123')

    expect(returnTo).toBe('/projects')
    expect(getAccessToken()).toBe('memory-only-token')
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock.mock.calls[1]?.[0]).toBe('https://issuer.example/connect/token')
    expect(localStorage.getItem('access_token')).toBeNull()
    expect(sessionStorage.getItem('access_token')).toBeNull()
  })
})
