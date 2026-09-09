/**
 * Build-time production CSP for the Vite web app.
 * connect-src is derived from VITE_API_BASE_URL so browser fetch targets stay aligned.
 */

export function parseConfiguredApiOrigin(apiBaseUrl) {
  const trimmed = String(apiBaseUrl ?? '').trim().replace(/\/$/, '')
  if (trimmed === '') {
    return ''
  }

  let parsed
  try {
    parsed = new URL(trimmed)
  } catch {
    throw new Error('VITE_API_BASE_URL must be an absolute http(s) API base URL')
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('VITE_API_BASE_URL must use http or https')
  }
  if (parsed.username || parsed.password) {
    throw new Error('VITE_API_BASE_URL must not include credentials')
  }
  if (parsed.pathname !== '' && parsed.pathname !== '/') {
    throw new Error('VITE_API_BASE_URL must be an origin or origin root without a path')
  }
  if (parsed.search || parsed.hash) {
    throw new Error('VITE_API_BASE_URL must not include query or fragment')
  }

  return parsed.origin
}

export function parseConfiguredOidcIssuer(oidcIssuer) {
  const trimmed = String(oidcIssuer ?? '').trim().replace(/\/$/, '')
  if (trimmed === '') {
    return ''
  }

  let parsed
  try {
    parsed = new URL(trimmed)
  } catch {
    throw new Error('VITE_OIDC_ISSUER must be an absolute http(s) issuer URL')
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('VITE_OIDC_ISSUER must use http or https')
  }

  return parsed.origin
}

export function buildProductionContentSecurityPolicy(apiBaseUrl, oidcIssuer = '') {
  const connectSources = ["'self'"]
  const apiOrigin = parseConfiguredApiOrigin(apiBaseUrl)
  if (apiOrigin) {
    connectSources.push(apiOrigin)
  }
  const issuerOrigin = parseConfiguredOidcIssuer(oidcIssuer)
  if (issuerOrigin) {
    connectSources.push(issuerOrigin)
  }

  return [
    "default-src 'self'",
    "script-src 'self'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "font-src 'self'",
    `connect-src ${connectSources.join(' ')}`,
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join('; ')
}

export function productionCspMetaTag(apiBaseUrl, oidcIssuer = '') {
  const content = buildProductionContentSecurityPolicy(apiBaseUrl, oidcIssuer)
  return `<meta http-equiv="Content-Security-Policy" content="${content}" />`
}
