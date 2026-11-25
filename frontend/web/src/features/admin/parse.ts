import type { ProductInput, ProductUpdateInput, VariantInput } from '@/shared/api/types'
import { isWholeAmount } from '@/shared/lib/format'

export type VariantDraft = {
  sku: string
  attributes: string
  shownPrice: string
  sellingPrice: string
  currency: string
  stock: string
  images: string
}

export const emptyVariant = (): VariantDraft => ({
  sku: '',
  attributes: '',
  shownPrice: '',
  sellingPrice: '',
  currency: 'INR',
  stock: '0',
  images: '',
})

export function splitList(value: string): string[] {
  return value
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean)
}

export function parseAttributes(value: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const pair of splitList(value)) {
    const [key, ...rest] = pair.split('=')
    const name = key?.trim()
    const text = rest.join('=').trim()
    if (!name || !text) throw new Error(`Attribute "${pair}" must look like key=value`)
    out[name] = text
  }
  return out
}

function parseAmount(value: string, label: string): number {
  const parsed = Number(value)
  if (value.trim() === '' || !Number.isFinite(parsed) || parsed < 0)
    throw new Error(`${label} must be a positive number`)
  return parsed
}

export function toVariantInput(draft: VariantDraft): VariantInput {
  const currency = draft.currency.trim().toUpperCase() || 'INR'
  const shown = parseAmount(draft.shownPrice, 'Shown price')
  const selling =
    draft.sellingPrice.trim() === '' ? shown : parseAmount(draft.sellingPrice, 'Selling price')
  const stock = Number(draft.stock || '0')
  if (!isWholeAmount(stock)) throw new Error('Stock must be a whole number')
  const variant: VariantInput = {
    sku: draft.sku.trim(),
    shown_price: shown,
    selling_price: selling,
    currency,
    stock_units: stock,
  }
  const attributes = parseAttributes(draft.attributes)
  if (Object.keys(attributes).length > 0) variant.attributes = attributes
  const images = splitList(draft.images)
  if (images.length > 0)
    variant.image_urls = images.map((image) =>
      /^[0-9A-HJKMNP-TV-Z]{26}$/i.test(image) ? `/v1/images/${image}` : image,
    )
  return variant
}

export function parseJSON(value: string): unknown {
  try {
    return JSON.parse(value)
  } catch {
    throw new Error('That is not valid JSON')
  }
}

function asArray(value: unknown, key: string): unknown[] {
  if (Array.isArray(value)) return value
  if (
    value &&
    typeof value === 'object' &&
    Array.isArray((value as Record<string, unknown>)[key])
  ) {
    return (value as Record<string, unknown[]>)[key] ?? []
  }
  throw new Error(`Expected an array or {"${key}": [...]}`)
}

export function parseProducts(value: string): ProductInput[] {
  return asArray(parseJSON(value), 'products') as ProductInput[]
}

export function parseProductUpdates(value: string): ProductUpdateInput[] {
  const items = asArray(parseJSON(value), 'products') as ProductUpdateInput[]
  if (items.some((item) => !item || typeof item.id !== 'string' || item.id === ''))
    throw new Error('Every product needs an "id"')
  return items
}
