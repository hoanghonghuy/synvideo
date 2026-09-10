export type OidcConfig = {
  issuer: string
  clientId: string
  audience: string
  scopes: string
  redirectUri: string
  allowedEndpointOrigins?: string[]
}

const DEFAULT_SCOPES = 'openid profile email'

function parseAllowedEndpointOrigins(raw: string): string[] {
  const origins = raw
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)

  return origins.map((value) => {
    let parsed: URL
    try {
      parsed = new URL(value)
    } catch {
      throw new Error('VITE_OIDC_CONNECT_ORIGINS entries must be absolute URLs')
    }
    if (parsed.protocol !== 'https:') {
      throw new Error('VITE_OIDC_CONNECT_ORIGINS entries must use https')
    }
    if (parsed.username || parsed.password || (parsed.pathname !== '' && parsed.pathname !== '/') || parsed.search || parsed.hash) {
      throw new Error('VITE_OIDC_CONNECT_ORIGINS entries must be origins without path, credentials, query, or fragment')
    }
    return parsed.origin
  })
}

export function loadOidcConfig(): OidcConfig | null {
  const issuer = (import.meta.env.VITE_OIDC_ISSUER ?? '').trim().replace(/\/$/, '')
  const clientId = (import.meta.env.VITE_OIDC_CLIENT_ID ?? '').trim()
  const audience = (import.meta.env.VITE_OIDC_AUDIENCE ?? '').trim()
  const scopes = (import.meta.env.VITE_OIDC_SCOPES ?? DEFAULT_SCOPES).trim() || DEFAULT_SCOPES
  const redirectUri = (import.meta.env.VITE_OIDC_REDIRECT_URI ?? '').trim()
  const allowedEndpointOrigins = parseAllowedEndpointOrigins(import.meta.env.VITE_OIDC_CONNECT_ORIGINS ?? '')

  if (!issuer || !clientId) {
    return null
  }

  let parsedIssuer: URL
  try {
    parsedIssuer = new URL(issuer)
  } catch {
    throw new Error('VITE_OIDC_ISSUER must be an absolute URL')
  }
  if (import.meta.env.PROD && parsedIssuer.protocol !== 'https:') {
    throw new Error('VITE_OIDC_ISSUER must use https in production')
  }
  if (parsedIssuer.protocol !== 'https:' && parsedIssuer.protocol !== 'http:') {
    throw new Error('VITE_OIDC_ISSUER must use http or https')
  }
  if (parsedIssuer.username || parsedIssuer.password || parsedIssuer.search || parsedIssuer.hash) {
    throw new Error('VITE_OIDC_ISSUER must not include credentials, query, or fragment')
  }

  return {
    issuer,
    clientId,
    audience,
    scopes,
    redirectUri: redirectUri || `${window.location.origin}/auth/callback`,
    allowedEndpointOrigins,
  }
}

export function isOidcConfigured(): boolean {
  return loadOidcConfig() !== null
}
