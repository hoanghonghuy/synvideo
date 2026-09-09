import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiUrl } from './http'

describe('apiUrl', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('keeps relative paths for same-origin development', () => {
    vi.stubEnv('VITE_API_BASE_URL', '')
    expect(apiUrl('/api/v1/healthz')).toBe('/api/v1/healthz')
  })

  it('prefixes configured production API origin', () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example.onrender.com')
    expect(apiUrl('/api/v1/readyz')).toBe('https://api.example.onrender.com/api/v1/readyz')
  })
})
