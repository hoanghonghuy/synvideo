import { isOidcConfigured } from '@/auth/config'
import { sanitizeReturnTo } from '@/auth/oidc'
import { clearAccessToken, getAccessToken } from '@/auth/session'

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

function currentReturnTo(): string {
  return sanitizeReturnTo(`${window.location.pathname}${window.location.search}${window.location.hash}`)
}

function transitionToReauth(): void {
  if (!isOidcConfigured()) {
    return
  }

  const returnTo = currentReturnTo()
  if (window.location.pathname === '/sign-in' || window.location.pathname === '/auth/callback') {
    return
  }

  const query = new URLSearchParams({
    reason: 'session-expired',
    returnTo,
  })
  window.location.assign(`/sign-in?${query.toString()}`)
}

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const accessToken = getAccessToken()
  let requestInit: RequestInit = {
    credentials: 'include',
    ...init,
  }

  if (accessToken) {
    const headers = new Headers(init.headers ?? {})
    if (!headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${accessToken}`)
    }
    requestInit = {
      ...requestInit,
      headers,
    }
  }

  const response = await fetch(apiUrl(path), requestInit)

  if (response.status === 401 && accessToken) {
    clearAccessToken()
    transitionToReauth()
  }

  return response
}
