import type { OidcConfig } from './config'

export type OidcDiscoveryDocument = {
  issuer: string
  authorizationEndpoint: string
  tokenEndpoint: string
}

type DiscoveryCacheEntry = {
  document: OidcDiscoveryDocument
  expiresAt: number
}

const DISCOVERY_CACHE_TTL_MS = 5 * 60 * 1000
const DISCOVERY_FETCH_TIMEOUT_MS = 5_000

const discoveryCache = new Map<string, DiscoveryCacheEntry>()

type DiscoveryResponse = {
  issuer?: string
  authorization_endpoint?: string
  token_endpoint?: string
}

export function resetOidcDiscoveryCache(): void {
  discoveryCache.clear()
}

export async function resolveOidcDiscovery(
  config: OidcConfig,
  fetchImpl: typeof fetch = fetch,
): Promise<OidcDiscoveryDocument> {
  const cached = discoveryCache.get(config.issuer)
  if (cached && cached.expiresAt > Date.now()) {
    return cached.document
  }

  const discoveryURL = `${config.issuer}/.well-known/openid-configuration`
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), DISCOVERY_FETCH_TIMEOUT_MS)

  let response: Response
  try {
    response = await fetchImpl(discoveryURL, {
      method: 'GET',
      headers: { Accept: 'application/json' },
      signal: controller.signal,
    })
  } catch {
    throw new Error('oidc discovery failed')
  } finally {
    clearTimeout(timeout)
  }

  if (!response.ok) {
    throw new Error('oidc discovery failed')
  }

  const payload = (await response.json()) as DiscoveryResponse
  const document = parseDiscoveryDocument(config, payload)

  discoveryCache.set(config.issuer, {
    document,
    expiresAt: Date.now() + DISCOVERY_CACHE_TTL_MS,
  })

  return document
}

function parseDiscoveryDocument(config: OidcConfig, payload: DiscoveryResponse): OidcDiscoveryDocument {
  const issuer = normalizeIssuer(payload.issuer ?? '')
  if (issuer !== config.issuer) {
    throw new Error('oidc discovery issuer mismatch')
  }

  const issuerURL = new URL(config.issuer)
  const allowedOrigins = new Set([issuerURL.origin, ...(config.allowedEndpointOrigins ?? [])])
  const requireHTTPS = issuerURL.protocol === 'https:'
  const authorizationEndpoint = validateEndpoint(
    payload.authorization_endpoint,
    'authorization_endpoint',
    allowedOrigins,
    requireHTTPS,
  )
  const tokenEndpoint = validateEndpoint(payload.token_endpoint, 'token_endpoint', allowedOrigins, requireHTTPS)

  return {
    issuer,
    authorizationEndpoint,
    tokenEndpoint,
  }
}

function normalizeIssuer(value: string): string {
  return value.trim().replace(/\/$/, '')
}

function validateEndpoint(
  value: string | undefined,
  fieldName: string,
  allowedOrigins: Set<string>,
  requireHTTPS: boolean,
): string {
  const raw = (value ?? '').trim()
  if (raw === '') {
    throw new Error(`oidc discovery missing ${fieldName}`)
  }

  let parsed: URL
  try {
    parsed = new URL(raw)
  } catch {
    throw new Error(`oidc discovery ${fieldName} must be an absolute URL`)
  }

  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
    throw new Error(`oidc discovery ${fieldName} must use http or https`)
  }
  if (requireHTTPS && parsed.protocol !== 'https:') {
    throw new Error(`oidc discovery ${fieldName} must use https for a secure issuer`)
  }
  if (parsed.username || parsed.password) {
    throw new Error(`oidc discovery ${fieldName} must not include credentials`)
  }
  if (parsed.hash) {
    throw new Error(`oidc discovery ${fieldName} must not include a fragment`)
  }
  if (!allowedOrigins.has(parsed.origin)) {
    throw new Error(`oidc discovery ${fieldName} origin is not allowed by VITE_OIDC_CONNECT_ORIGINS`)
  }

  return parsed.toString()
}
