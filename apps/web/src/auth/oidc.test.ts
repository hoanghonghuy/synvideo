import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { clearAccessToken, getAccessToken, setAccessToken } from './session'

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
