import { apiBase } from './config'
import { ApiError } from './errors'
import type { Session } from './types'

type Body = Record<string, unknown> | unknown[] | FormData

export type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  body?: Body
  auth?: boolean
  adminKey?: string
  signal?: AbortSignal
}

export type TokenProvider = {
  accessToken: () => string | null
  refreshToken: () => string | null
  onRefreshed: (session: Session) => void
  onExpired: () => void
}

let tokens: TokenProvider | null = null
let refreshing: Promise<string | null> | null = null

export function configureTokens(provider: TokenProvider): void {
  tokens = provider
}

async function parseError(response: Response): Promise<ApiError> {
  const retry = Number(response.headers.get('Retry-After'))
  const retryAfter = Number.isFinite(retry) && retry > 0 ? retry : null
  try {
    const data: unknown = await response.json()
    if (data && typeof data === 'object' && 'code' in data && 'message' in data) {
      const { code, message } = data as { code: unknown; message: unknown }
      if (typeof code === 'string' && typeof message === 'string') {
        return new ApiError(response.status, code, message, retryAfter)
      }
    }
  } catch {
    return new ApiError(
      response.status,
      'http_error',
      response.statusText || 'Request failed',
      retryAfter,
    )
  }
  return new ApiError(
    response.status,
    'http_error',
    response.statusText || 'Request failed',
    retryAfter,
  )
}

async function send(
  path: string,
  options: RequestOptions,
  accessToken: string | null,
): Promise<Response> {
  const headers = new Headers()
  let body: BodyInit | undefined
  if (options.body instanceof FormData) {
    body = options.body
  } else if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json')
    body = JSON.stringify(options.body)
  }
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`)
  if (options.adminKey) headers.set('X-Admin-Key', options.adminKey)
  const init: RequestInit = { method: options.method ?? 'GET', headers, credentials: 'omit' }
  if (body !== undefined) init.body = body
  if (options.signal) init.signal = options.signal
  return fetch(`${apiBase}${path}`, init)
}

export function refreshSession(): Promise<string | null> {
  if (refreshing) return refreshing
  const provider = tokens
  const refreshToken = provider?.refreshToken()
  if (!provider || !refreshToken) return Promise.resolve(null)
  refreshing = (async () => {
    try {
      const response = await send(
        '/users/token/refresh',
        { method: 'POST', body: { refresh_token: refreshToken } },
        null,
      )
      if (!response.ok) {
        if (response.status === 401) provider.onExpired()
        return null
      }
      const session = (await response.json()) as Session
      provider.onRefreshed(session)
      return session.access_token
    } catch {
      return null
    } finally {
      refreshing = null
    }
  })()
  return refreshing
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  let accessToken = options.auth ? (tokens?.accessToken() ?? null) : null
  if (options.auth && !accessToken) accessToken = await refreshSession()
  if (options.auth && !accessToken)
    throw new ApiError(401, 'unauthorized', 'Please sign in to continue.')
  let response = await send(path, options, accessToken)
  if (response.status === 401 && options.auth) {
    const renewed = await refreshSession()
    if (renewed) response = await send(path, options, renewed)
  }
  if (!response.ok) throw await parseError(response)
  return (await response.json()) as T
}

export async function requestBlob(path: string, options: RequestOptions = {}): Promise<Blob> {
  const response = await send(path, options, null)
  if (!response.ok) throw await parseError(response)
  return response.blob()
}
