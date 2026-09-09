export type OidcConfig = {
  issuer: string
  clientId: string
  audience: string
  scopes: string
  redirectUri: string
}

const DEFAULT_SCOPES = 'openid profile email'

export function loadOidcConfig(): OidcConfig | null {
  const issuer = (import.meta.env.VITE_OIDC_ISSUER ?? '').trim().replace(/\/$/, '')
  const clientId = (import.meta.env.VITE_OIDC_CLIENT_ID ?? '').trim()
  const audience = (import.meta.env.VITE_OIDC_AUDIENCE ?? '').trim()
  const scopes = (import.meta.env.VITE_OIDC_SCOPES ?? DEFAULT_SCOPES).trim() || DEFAULT_SCOPES
  const redirectUri = (import.meta.env.VITE_OIDC_REDIRECT_URI ?? '').trim()

  if (!issuer || !clientId) {
    return null
  }

  return {
    issuer,
    clientId,
    audience,
    scopes,
    redirectUri: redirectUri || `${window.location.origin}/auth/callback`,
  }
}

export function isOidcConfigured(): boolean {
  return loadOidcConfig() !== null
}
