const defaultPort = '3000'

function readPort(): string {
  const meta = document.querySelector<HTMLMetaElement>('meta[name="ai-api-port"]')
  const value = meta?.content.trim() ?? ''
  return /^\d{1,5}$/.test(value) ? value : defaultPort
}

export const apiOrigin = `${window.location.protocol}//${window.location.hostname}:${readPort()}`

export const apiBase = `${apiOrigin}/v1`

const ulid = /^[0-9A-HJKMNP-TV-Z]{26}$/i

export function resolveImageURL(value: string | undefined): string | undefined {
  if (!value) return undefined
  const trimmed = value.trim()
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  if (trimmed.startsWith('/')) return `${apiOrigin}${trimmed}`
  if (ulid.test(trimmed)) return `${apiBase}/images/${trimmed}`
  return undefined
}
