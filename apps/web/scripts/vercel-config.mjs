import { buildProductionContentSecurityPolicy } from './production-csp.mjs'

export const VERCEL_MONOREPO_CONTRACT = {
  rootDirectory: '.',
  configPath: 'vercel.mjs',
  lockfilePath: 'package-lock.json',
  workspacePath: 'apps/web',
  installCommand: 'npm ci',
  buildCommand: 'npm run build:web',
  outputDirectory: 'apps/web/dist',
}

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

export function buildVercelSecurityHeaders(apiBaseUrl, oidcIssuer = '') {
  const csp = buildProductionContentSecurityPolicy(apiBaseUrl, oidcIssuer)
  return [
    { key: 'X-Content-Type-Options', value: 'nosniff' },
    { key: 'X-Frame-Options', value: 'DENY' },
    { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
    { key: 'Content-Security-Policy', value: csp },
  ]
}

export function buildVercelDeploymentConfig(apiBaseUrl, { requireApiBaseUrl = true, oidcIssuer = '' } = {}) {
  const resolved = requireApiBaseUrl
    ? requireConfiguredApiBaseUrl(apiBaseUrl)
    : String(apiBaseUrl ?? '').trim()

  return {
    installCommand: VERCEL_MONOREPO_CONTRACT.installCommand,
    buildCommand: VERCEL_MONOREPO_CONTRACT.buildCommand,
    outputDirectory: VERCEL_MONOREPO_CONTRACT.outputDirectory,
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
        headers: buildVercelSecurityHeaders(resolved, oidcIssuer),
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
