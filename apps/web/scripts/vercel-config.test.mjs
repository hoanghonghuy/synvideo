import assert from 'node:assert/strict'
import test from 'node:test'

import {
  VERCEL_MONOREPO_CONTRACT,
  buildVercelDeploymentConfig,
  buildVercelSecurityHeaders,
  getVercelContentSecurityPolicy,
  requireConfiguredApiBaseUrl,
} from './vercel-config.mjs'

test('requireConfiguredApiBaseUrl rejects empty API base URLs', () => {
  assert.throws(() => requireConfiguredApiBaseUrl(''), /VITE_API_BASE_URL is required/)
  assert.throws(() => requireConfiguredApiBaseUrl('   '), /VITE_API_BASE_URL is required/)
})

test('buildVercelSecurityHeaders includes configured API origin in connect-src', () => {
  const headers = buildVercelSecurityHeaders('https://api.custom.example')
  const csp = headers.find((header) => header.key === 'Content-Security-Policy')?.value
  assert.match(csp, /connect-src 'self' https:\/\/api\.custom\.example/)
  assert.doesNotMatch(csp, /onrender\.com/)
  assert.doesNotMatch(csp, /\*/)
})

test('buildVercelDeploymentConfig materializes required security headers', () => {
  const config = buildVercelDeploymentConfig('https://api.custom.example')
  const csp = getVercelContentSecurityPolicy(config)
  assert.match(csp, /connect-src 'self' https:\/\/api\.custom\.example/)
  assert.equal(config.installCommand, VERCEL_MONOREPO_CONTRACT.installCommand)
  assert.equal(config.buildCommand, VERCEL_MONOREPO_CONTRACT.buildCommand)
  assert.equal(config.outputDirectory, VERCEL_MONOREPO_CONTRACT.outputDirectory)
  assert.deepEqual(config.rewrites, [{ source: '/(.*)', destination: '/index.html' }])
})

test('VERCEL_MONOREPO_CONTRACT targets repository-root workspace install and build', () => {
  assert.equal(VERCEL_MONOREPO_CONTRACT.rootDirectory, '.')
  assert.equal(VERCEL_MONOREPO_CONTRACT.configPath, 'vercel.mjs')
  assert.equal(VERCEL_MONOREPO_CONTRACT.lockfilePath, 'package-lock.json')
  assert.equal(VERCEL_MONOREPO_CONTRACT.workspacePath, 'apps/web')
  assert.equal(VERCEL_MONOREPO_CONTRACT.installCommand, 'npm ci')
  assert.equal(VERCEL_MONOREPO_CONTRACT.buildCommand, 'npm run build:web')
  assert.equal(VERCEL_MONOREPO_CONTRACT.outputDirectory, 'apps/web/dist')
})

test('buildVercelDeploymentConfig fails when API base URL is required but missing', () => {
  assert.throws(() => buildVercelDeploymentConfig(''), /VITE_API_BASE_URL is required/)
})
