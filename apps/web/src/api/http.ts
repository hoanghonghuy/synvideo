import { getAccessToken } from '@/auth/session'

function configuredApiBase(): string {
  return (import.meta.env.VITE_API_BASE_URL ?? '').trim().replace(/\/$/, '')
}

export function apiUrl(path: string): string {
  if (!path.startsWith('/')) {
    throw new Error(`API path must start with "/": ${path}`)
  }
  const base = configuredApiBase()
  return base ? `${base}${path}` : path
}

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const accessToken = getAccessToken()
  if (!accessToken) {
    return fetch(apiUrl(path), {
      credentials: 'include',
      ...init,
    })
  }

  const headers = new Headers(init.headers ?? {})
  if (!headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${accessToken}`)
  }

  return fetch(apiUrl(path), {
    credentials: 'include',
    ...init,
    headers,
  })
}
