/**
 * Build-time production CSP for the Vite web app.
 * connect-src is derived from the API origin and the same bounded OIDC origin
 * allowlist used by runtime discovery validation.
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
    throw new Error('VITE_OIDC_ISSUER must be an absolute https issuer URL')
  }

  if (parsed.protocol !== 'https:') {
    throw new Error('VITE_OIDC_ISSUER must use https in production')
  }
  if (parsed.username || parsed.password || parsed.search || parsed.hash) {
    throw new Error('VITE_OIDC_ISSUER must not include credentials, query, or fragment')
  }

  return parsed.origin
}

export function parseConfiguredOidcConnectOrigins(rawOrigins) {
  const raw = String(rawOrigins ?? '').trim()
  if (raw === '') {
    return []
  }

  return raw.split(',').map((entry) => {
    const trimmed = entry.trim()
    let parsed
    try {
      parsed = new URL(trimmed)
    } catch {
      throw new Error('VITE_OIDC_CONNECT_ORIGINS entries must be absolute https origins')
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

export function buildProductionContentSecurityPolicy(apiBaseUrl, oidcIssuer = '', oidcConnectOrigins = '') {
  const connectSources = ["'self'"]
  const apiOrigin = parseConfiguredApiOrigin(apiBaseUrl)
  if (apiOrigin) {
    connectSources.push(apiOrigin)
  }
  const issuerOrigin = parseConfiguredOidcIssuer(oidcIssuer)
  if (issuerOrigin) {
    connectSources.push(issuerOrigin)
  }
  for (const origin of parseConfiguredOidcConnectOrigins(oidcConnectOrigins)) {
    if (!connectSources.includes(origin)) {
      connectSources.push(origin)
    }
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

export function productionCspMetaTag(apiBaseUrl, oidcIssuer = '', oidcConnectOrigins = '') {
  const content = buildProductionContentSecurityPolicy(apiBaseUrl, oidcIssuer, oidcConnectOrigins)
  return `<meta http-equiv="Content-Security-Policy" content="${content}" />`
}
