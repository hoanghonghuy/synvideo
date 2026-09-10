import { resolveOidcDiscovery } from './discovery'
import { createCodeChallenge, createCodeVerifier, createOAuthState } from './pkce'
import { loadOidcConfig, type OidcConfig } from './config'
import { clearAccessToken, setAccessToken } from './session'

const FLOW_STATE_KEY = 'synvideo_oidc_flow_state'

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

export function beginSignIn(returnTo = '/projects'): void {
  const config = loadOidcConfig()
  if (!config) {
    throw new Error('OIDC is not configured')
  }

  void startAuthorizationRedirect(config, returnTo)
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
  const code = params.get('code')
  const state = params.get('state')
  if (!code || !state) {
    throw new Error('missing authorization code')
  }

  const pendingRaw = sessionStorage.getItem(FLOW_STATE_KEY)
  sessionStorage.removeItem(FLOW_STATE_KEY)
  if (!pendingRaw) {
    throw new Error('missing oauth flow state')
  }

  const pending = JSON.parse(pendingRaw) as PendingFlow
  if (pending.state !== state) {
    throw new Error('oauth state mismatch')
  }

  const tokenResponse = await exchangeAuthorizationCode(config, code, pending.codeVerifier)
  if (!tokenResponse.access_token) {
    throw new Error('token response missing access_token')
  }

  setAccessToken(tokenResponse.access_token)
  return pending.returnTo || '/projects'
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
