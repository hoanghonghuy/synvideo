import { resolveOidcDiscovery } from './discovery'
import { createCodeChallenge, createCodeVerifier, createOAuthState } from './pkce'
import { loadOidcConfig, type OidcConfig } from './config'
import { clearAccessToken, setAccessToken } from './session'

const FLOW_STATE_KEY = 'synvideo_oidc_flow_state'
const DEFAULT_RETURN_TO = '/projects'

type PendingFlow = {
  codeVerifier: string
  state: string
  returnTo: string
}

type TokenResponse = {
  access_token?: string
  expires_in?: number
  token_type?: string
}

export function sanitizeReturnTo(returnTo: string | null | undefined): string {
  const candidate = (returnTo ?? '').trim()
  if (!candidate.startsWith('/') || candidate.startsWith('//')) {
    return DEFAULT_RETURN_TO
  }

  try {
    const parsed = new URL(candidate, window.location.origin)
    if (parsed.origin !== window.location.origin) {
      return DEFAULT_RETURN_TO
    }
    if (parsed.pathname === '/auth/callback') {
      return DEFAULT_RETURN_TO
    }
    return `${parsed.pathname}${parsed.search}${parsed.hash}`
  } catch {
    return DEFAULT_RETURN_TO
  }
}

export async function beginSignIn(returnTo = DEFAULT_RETURN_TO): Promise<void> {
  const config = loadOidcConfig()
  if (!config) {
    throw new Error('OIDC is not configured')
  }

  await startAuthorizationRedirect(config, sanitizeReturnTo(returnTo))
}

async function startAuthorizationRedirect(config: OidcConfig, returnTo: string): Promise<void> {
  const discovery = await resolveOidcDiscovery(config)
  const codeVerifier = createCodeVerifier()
  const state = createOAuthState()
  const challenge = await createCodeChallenge(codeVerifier)

  sessionStorage.setItem(
    FLOW_STATE_KEY,
    JSON.stringify({ codeVerifier, state, returnTo } satisfies PendingFlow),
  )

  const params = new URLSearchParams({
    response_type: 'code',
    client_id: config.clientId,
    redirect_uri: config.redirectUri,
    scope: config.scopes,
    state,
    code_challenge: challenge,
    code_challenge_method: 'S256',
  })
  if (config.audience) {
    params.set('audience', config.audience)
  }

  const separator = discovery.authorizationEndpoint.includes('?') ? '&' : '?'
  window.location.assign(`${discovery.authorizationEndpoint}${separator}${params.toString()}`)
}

export async function completeSignInFromCallback(search: string): Promise<string> {
  const config = loadOidcConfig()
  if (!config) {
    throw new Error('OIDC is not configured')
  }

  const params = new URLSearchParams(search.startsWith('?') ? search.slice(1) : search)
  const providerError = params.get('error')
  if (providerError) {
    sessionStorage.removeItem(FLOW_STATE_KEY)
    throw new Error(providerError === 'access_denied' ? 'sign-in was cancelled' : 'identity provider rejected sign-in')
  }

  const code = params.get('code')
  const state = params.get('state')
  if (!code || !state) {
    sessionStorage.removeItem(FLOW_STATE_KEY)
    throw new Error('missing authorization code')
  }

  const pendingRaw = sessionStorage.getItem(FLOW_STATE_KEY)
  sessionStorage.removeItem(FLOW_STATE_KEY)
  if (!pendingRaw) {
    throw new Error('missing oauth flow state')
  }

  let pending: PendingFlow
  try {
    pending = JSON.parse(pendingRaw) as PendingFlow
  } catch {
    throw new Error('invalid oauth flow state')
  }
  if (!pending.codeVerifier || !pending.state || pending.state !== state) {
    throw new Error('oauth state mismatch')
  }

  const tokenResponse = await exchangeAuthorizationCode(config, code, pending.codeVerifier)
  if (!tokenResponse.access_token) {
    throw new Error('token response missing access_token')
  }

  setAccessToken(tokenResponse.access_token)
  return sanitizeReturnTo(pending.returnTo)
}

export function signOut(): void {
  clearAccessToken()
  sessionStorage.removeItem(FLOW_STATE_KEY)
}

async function exchangeAuthorizationCode(
  config: OidcConfig,
  code: string,
  codeVerifier: string,
): Promise<TokenResponse> {
  const discovery = await resolveOidcDiscovery(config)
  const body = new URLSearchParams({
    grant_type: 'authorization_code',
    client_id: config.clientId,
    code,
    redirect_uri: config.redirectUri,
    code_verifier: codeVerifier,
  })

  const response = await fetch(discovery.tokenEndpoint, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    body,
  })

  if (!response.ok) {
    throw new Error('token exchange failed')
  }

  return (await response.json()) as TokenResponse
}
