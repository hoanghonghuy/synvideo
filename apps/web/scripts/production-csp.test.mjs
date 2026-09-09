import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildProductionContentSecurityPolicy,
  parseConfiguredApiOrigin,
  productionCspMetaTag,
} from './production-csp.mjs'

test('parseConfiguredApiOrigin accepts origin root URLs', () => {
  assert.equal(parseConfiguredApiOrigin('https://api.example.onrender.com'), 'https://api.example.onrender.com')
  assert.equal(parseConfiguredApiOrigin('https://api.example.onrender.com/'), 'https://api.example.onrender.com')
})

test('parseConfiguredApiOrigin rejects pathful API bases', () => {
  assert.throws(() => parseConfiguredApiOrigin('https://api.example.com/v1'))
})

test('buildProductionContentSecurityPolicy includes configured API origin in connect-src', () => {
  const csp = buildProductionContentSecurityPolicy('https://api.custom.example')
  assert.match(csp, /connect-src 'self' https:\/\/api\.custom\.example/)
  assert.doesNotMatch(csp, /onrender\.com/)
  assert.doesNotMatch(csp, /\*/)
})

test('buildProductionContentSecurityPolicy omits provider-specific hosts when unset', () => {
  const csp = buildProductionContentSecurityPolicy('')
  assert.equal(csp.includes("connect-src 'self'"), true)
  assert.doesNotMatch(csp, /onrender\.com/)
})

test('productionCspMetaTag renders HTML meta injection', () => {
  const tag = productionCspMetaTag('https://api.custom.example')
  assert.match(tag, /^<meta http-equiv="Content-Security-Policy" content=".*connect-src 'self' https:\/\/api\.custom\.example.*" \/>$/)
})

