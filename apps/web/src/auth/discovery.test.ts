import { afterEach, describe, expect, it, vi } from 'vitest'

import type { OidcConfig } from './config'
import { resetOidcDiscoveryCache, resolveOidcDiscovery } from './discovery'

const baseConfig: OidcConfig = {
  issuer: 'https://issuer.example',
  clientId: 'web-client',
  audience: 'synvideo-api',
  scopes: 'openid profile email',
  redirectUri: 'https://app.example/auth/callback',
}

describe('resolveOidcDiscovery', () => {
  afterEach(() => {
    resetOidcDiscoveryCache()
    vi.restoreAllMocks()
  })

  it('uses advertised authorization and token endpoints from discovery', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      issuer: 'https://issuer.example',
      authorization_endpoint: 'https://issuer.example/connect/authorize',
      token_endpoint: 'https://issuer.example/connect/token',
    }), { status: 200 }))

    const discovery = await resolveOidcDiscovery(baseConfig, fetchMock)

    expect(fetchMock).toHaveBeenCalledWith(
      'https://issuer.example/.well-known/openid-configuration',
      expect.objectContaining({ method: 'GET' }),
    )
    expect(discovery.authorizationEndpoint).toBe('https://issuer.example/connect/authorize')
    expect(discovery.tokenEndpoint).toBe('https://issuer.example/connect/token')
  })

  it('caches discovery within the ttl window', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      issuer: 'https://issuer.example',
      authorization_endpoint: 'https://issuer.example/oauth2/auth',
      token_endpoint: 'https://issuer.example/oauth2/token',
    }), { status: 200 }))

    await resolveOidcDiscovery(baseConfig, fetchMock)
    await resolveOidcDiscovery(baseConfig, fetchMock)

    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('rejects issuer mismatch fail-closed', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      issuer: 'https://evil.example',
      authorization_endpoint: 'https://evil.example/connect/authorize',
      token_endpoint: 'https://evil.example/connect/token',
    }), { status: 200 }))

    await expect(resolveOidcDiscovery(baseConfig, fetchMock)).rejects.toThrow('issuer mismatch')
  })
})
