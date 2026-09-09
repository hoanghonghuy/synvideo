import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { buildProductionContentSecurityPolicy } from './production-csp.mjs'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const defaultVercelPath = join(scriptDir, '..', 'vercel.json')

export function applyVercelContentSecurityPolicy(vercelConfig, apiBaseUrl) {
  const csp = buildProductionContentSecurityPolicy(apiBaseUrl)
  const headersGroups = vercelConfig.headers ?? []

  for (const group of headersGroups) {
    const headers = group.headers ?? []
    const existing = headers.find((header) => header.key === 'Content-Security-Policy')
    if (existing) {
      existing.value = csp
      return vercelConfig
    }
  }

  if (headersGroups.length === 0) {
    vercelConfig.headers = [{ source: '/(.*)', headers: [] }]
  }

  const targetGroup = vercelConfig.headers[0]
  targetGroup.headers = targetGroup.headers ?? []
  targetGroup.headers.push({ key: 'Content-Security-Policy', value: csp })
  return vercelConfig
}

export function syncVercelContentSecurityPolicy({
  apiBaseUrl = process.env.VITE_API_BASE_URL ?? '',
  vercelPath = defaultVercelPath,
  requireApiBaseUrl = false,
} = {}) {
  const trimmed = String(apiBaseUrl ?? '').trim()
  if (trimmed === '') {
    if (requireApiBaseUrl) {
      throw new Error('VITE_API_BASE_URL is required to materialize the Vercel CSP response header')
    }
    return null
  }

  const vercelConfig = JSON.parse(readFileSync(vercelPath, 'utf8'))
  const updated = applyVercelContentSecurityPolicy(vercelConfig, trimmed)
  writeFileSync(vercelPath, `${JSON.stringify(updated, null, 2)}\n`)
  return updated
}

const isMainModule = process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)

if (isMainModule) {
  const requireApiBaseUrl = process.argv.includes('--require-api-base-url')
  try {
    syncVercelContentSecurityPolicy({ requireApiBaseUrl })
  } catch (error) {
    console.error(error instanceof Error ? error.message : error)
    process.exit(1)
  }
}
