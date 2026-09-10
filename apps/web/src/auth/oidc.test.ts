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
      allowedEndpointOrigins: [],
    })
  })

  it('parses a bounded HTTPS endpoint-origin allowlist', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    vi.stubEnv('VITE_OIDC_CONNECT_ORIGINS', 'https://login.example, https://keys.example/')
    const { loadOidcConfig } = await import('./config')

    expect(loadOidcConfig()?.allowedEndpointOrigins).toEqual([
      'https://login.example',
      'https://keys.example',
    ])
  })
})

describe('oidc discovery-backed flow', () => {
  beforeEach(() => {
    vi.stubGlobal('location', { origin: 'https://app.example', assign: vi.fn() })
  })

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
    vi.stubGlobal('location', { origin: 'https://app.example', assign })

    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      issuer: 'https://issuer.example',
      authorization_endpoint: 'https://issuer.example/connect/authorize',
      token_endpoint: 'https://issuer.example/connect/token',
    }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const { beginSignIn } = await import('./oidc')
    await beginSignIn('/projects')

    expect(assign).toHaveBeenCalled()

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

  it('restores a same-app destination including query and hash', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    sessionStorage.setItem('synvideo_oidc_flow_state', JSON.stringify({
      codeVerifier: 'verifier-123',
      state: 'state-123',
      returnTo: '/projects/project-1/scene-editor?tab=timeline#clip-3',
    }))
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({
        issuer: 'https://issuer.example',
        authorization_endpoint: 'https://issuer.example/connect/authorize',
        token_endpoint: 'https://issuer.example/connect/token',
      }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'token' }), { status: 200 })))

    const { completeSignInFromCallback } = await import('./oidc')
    await expect(completeSignInFromCallback('?code=auth-code&state=state-123'))
      .resolves.toBe('/projects/project-1/scene-editor?tab=timeline#clip-3')
  })

  it('rejects external and callback return destinations', async () => {
    const { sanitizeReturnTo } = await import('./oidc')
    expect(sanitizeReturnTo('https://evil.example/phish')).toBe('/projects')
    expect(sanitizeReturnTo('//evil.example/phish')).toBe('/projects')
    expect(sanitizeReturnTo('/auth/callback?loop=1')).toBe('/projects')
    expect(sanitizeReturnTo('/projects/new')).toBe('/projects/new')
  })

  it('surfaces provider cancellation and clears stale flow state', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    sessionStorage.setItem('synvideo_oidc_flow_state', JSON.stringify({
      codeVerifier: 'verifier-123',
      state: 'state-123',
      returnTo: '/projects',
    }))

    const { completeSignInFromCallback } = await import('./oidc')
    await expect(completeSignInFromCallback('?error=access_denied&state=state-123'))
      .rejects.toThrow('sign-in was cancelled')
    expect(sessionStorage.getItem('synvideo_oidc_flow_state')).toBeNull()
  })

  it('fails closed on malformed saved oauth state and consumes the pending flow', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    sessionStorage.setItem('synvideo_oidc_flow_state', '{not-json')

    const { completeSignInFromCallback } = await import('./oidc')
    await expect(completeSignInFromCallback('?code=auth-code&state=state-123'))
      .rejects.toThrow('invalid oauth flow state')
    expect(sessionStorage.getItem('synvideo_oidc_flow_state')).toBeNull()
  })

  it('fails closed on oauth state mismatch and consumes the pending flow', async () => {
    vi.stubEnv('VITE_OIDC_ISSUER', 'https://issuer.example')
    vi.stubEnv('VITE_OIDC_CLIENT_ID', 'web-client')
    sessionStorage.setItem('synvideo_oidc_flow_state', JSON.stringify({
      codeVerifier: 'verifier-123',
      state: 'expected-state',
      returnTo: '/projects',
    }))

    const { completeSignInFromCallback } = await import('./oidc')
    await expect(completeSignInFromCallback('?code=auth-code&state=unexpected-state'))
      .rejects.toThrow('oauth state mismatch')
    expect(sessionStorage.getItem('synvideo_oidc_flow_state')).toBeNull()
  })
})
