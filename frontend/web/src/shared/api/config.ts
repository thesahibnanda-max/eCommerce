const defaultPort = '3000'
const sameOrigin = 'same-origin'

function readAPIOrigin(): string {
  const meta = document.querySelector<HTMLMetaElement>('meta[name="ai-api-port"]')
  const value = meta?.content.trim() ?? ''
  if (value === sameOrigin) return window.location.origin
  const port = /^\d{1,5}$/.test(value) ? value : defaultPort
  return `${window.location.protocol}//${window.location.hostname}:${port}`
}

export const apiOrigin = readAPIOrigin()

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
