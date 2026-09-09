import assert from 'node:assert/strict'
import test from 'node:test'

import {
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
  assert.equal(config.installCommand, 'npm ci')
  assert.equal(config.buildCommand, 'npm run build')
  assert.equal(config.outputDirectory, 'dist')
  assert.deepEqual(config.rewrites, [{ source: '/(.*)', destination: '/index.html' }])
})

test('buildVercelDeploymentConfig fails when API base URL is required but missing', () => {
  assert.throws(() => buildVercelDeploymentConfig(''), /VITE_API_BASE_URL is required/)
})
