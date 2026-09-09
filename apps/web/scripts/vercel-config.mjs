import { buildProductionContentSecurityPolicy } from './production-csp.mjs'

export const REQUIRED_SECURITY_HEADER_KEYS = [
  'X-Content-Type-Options',
  'X-Frame-Options',
  'Referrer-Policy',
  'Content-Security-Policy',
]

export function requireConfiguredApiBaseUrl(apiBaseUrl) {
  const trimmed = String(apiBaseUrl ?? '').trim()
  if (trimmed === '') {
    throw new Error('VITE_API_BASE_URL is required to materialize the Vercel deployment configuration CSP')
  }
  return trimmed
}

export function buildVercelSecurityHeaders(apiBaseUrl) {
  const csp = buildProductionContentSecurityPolicy(apiBaseUrl)
  return [
    { key: 'X-Content-Type-Options', value: 'nosniff' },
    { key: 'X-Frame-Options', value: 'DENY' },
    { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
    { key: 'Content-Security-Policy', value: csp },
  ]
}

export function buildVercelDeploymentConfig(apiBaseUrl, { requireApiBaseUrl = true } = {}) {
  const resolved = requireApiBaseUrl
    ? requireConfiguredApiBaseUrl(apiBaseUrl)
    : String(apiBaseUrl ?? '').trim()

  return {
    installCommand: 'npm ci',
    buildCommand: 'npm run build',
    outputDirectory: 'dist',
    framework: null,
    rewrites: [
      {
        source: '/(.*)',
        destination: '/index.html',
      },
    ],
    headers: [
      {
        source: '/(.*)',
        headers: buildVercelSecurityHeaders(resolved),
      },
    ],
  }
}

export function getVercelContentSecurityPolicy(config) {
  for (const group of config.headers ?? []) {
    for (const header of group.headers ?? []) {
      if (header.key === 'Content-Security-Policy') {
        return header.value ?? ''
      }
    }
  }
  return ''
}
