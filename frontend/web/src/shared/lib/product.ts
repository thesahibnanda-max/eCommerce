import type { Product, Variant } from '@/shared/api/types'

export function primaryVariant(product: Product): Variant | undefined {
  const inStock = product.variants.filter((v) => v.stock_units > 0)
  const pool = inStock.length > 0 ? inStock : product.variants
  return pool.reduce<Variant | undefined>(
    (best, v) => (!best || v.selling_price < best.selling_price ? v : best),
    undefined,
  )
}

export function productImage(product: Product): string | undefined {
  for (const variant of product.variants) {
    const url = variant.image_urls?.[0]
    if (url) return url
  }
  return undefined
}

export function productImages(product: Product, selected: Variant | undefined): string[] {
  const urls = new Set<string>()
  for (const url of selected?.image_urls ?? []) urls.add(url)
  for (const variant of product.variants) for (const url of variant.image_urls ?? []) urls.add(url)
  return [...urls]
}

const newForMs = 14 * 24 * 60 * 60 * 1000

export function isNewArrival(product: Product, now: number = Date.now()): boolean {
  const created = Date.parse(product.created_at)
  return Number.isFinite(created) && now - created >= 0 && now - created < newForMs
}
