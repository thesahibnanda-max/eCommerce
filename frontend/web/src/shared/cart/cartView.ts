import type { Cart, CartTotal } from '@/shared/api/types'
import type { LocalLine } from './localCartStore'
import { capFor } from './localCartStore'

export type CartLineView = {
  variantId: string
  productId: string | null
  name: string
  sku: string
  imageUrl: string | undefined
  attributes: Record<string, string>
  shownPrice: number
  sellingPrice: number
  currency: string
  stock: number
  quantity: number
  maxQuantity: number
  available: boolean
}

export type CartView = { lines: CartLineView[]; itemCount: number; totals: CartTotal[] }

export const emptyCartView: CartView = { lines: [], itemCount: 0, totals: [] }

export function fromServerCart(cart: Cart): CartView {
  return {
    lines: cart.items.map((item) => ({
      variantId: item.variant_id,
      productId: item.product_id ?? null,
      name: item.name ?? 'Unavailable item',
      sku: item.sku ?? '',
      imageUrl: item.image_url,
      attributes: item.attributes ?? {},
      shownPrice: item.shown_price,
      sellingPrice: item.selling_price,
      currency: item.currency ?? '',
      stock: item.stock_units,
      quantity: item.quantity,
      maxQuantity: capFor(item.stock_units),
      available: item.available,
    })),
    itemCount: cart.item_count,
    totals: cart.totals,
  }
}

export function fromLocalLines(lines: LocalLine[]): CartView {
  const totals = new Map<string, CartTotal>()
  let itemCount = 0
  const views = lines.map((line) => {
    const { snapshot } = line
    const available = snapshot.stock >= line.quantity
    itemCount += line.quantity
    if (available) {
      const total = totals.get(snapshot.currency) ?? {
        currency: snapshot.currency,
        shown_price: 0,
        selling_price: 0,
      }
      total.shown_price += snapshot.shownPrice * line.quantity
      total.selling_price += snapshot.sellingPrice * line.quantity
      totals.set(snapshot.currency, total)
    }
    return {
      variantId: line.variantId,
      productId: snapshot.productId,
      name: snapshot.name,
      sku: snapshot.sku,
      imageUrl: snapshot.imageUrl,
      attributes: snapshot.attributes ?? {},
      shownPrice: snapshot.shownPrice,
      sellingPrice: snapshot.sellingPrice,
      currency: snapshot.currency,
      stock: snapshot.stock,
      quantity: line.quantity,
      maxQuantity: capFor(snapshot.stock),
      available,
    }
  })
  return {
    lines: views,
    itemCount,
    totals: [...totals.values()].sort((a, b) => a.currency.localeCompare(b.currency)),
  }
}
