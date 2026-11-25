import { create } from 'zustand'
import { readJSON, writeStorage } from '@/shared/lib/safeStorage'

export const maxQuantityPerItem = 10
export const maxCartItems = 50

const storageKey = 'ai.cart'

export type LocalSnapshot = {
  productId: string
  name: string
  sku: string
  imageUrl?: string
  attributes?: Record<string, string>
  shownPrice: number
  sellingPrice: number
  currency: string
  stock: number
}

export type LocalLine = {
  variantId: string
  quantity: number
  addedAt: string
  snapshot: LocalSnapshot
}

type LocalCartState = {
  lines: LocalLine[]
  add: (
    variantId: string,
    snapshot: LocalSnapshot,
    quantity: number,
  ) => 'added' | 'capped' | 'full' | 'out_of_stock'
  setQuantity: (variantId: string, quantity: number) => void
  remove: (variantId: string) => void
  clear: () => void
}

function isLine(value: unknown): value is LocalLine {
  if (!value || typeof value !== 'object') return false
  const line = value as Record<string, unknown>
  const snapshot = line.snapshot as Record<string, unknown> | undefined
  return (
    typeof line.variantId === 'string' &&
    typeof line.quantity === 'number' &&
    typeof line.addedAt === 'string' &&
    !!snapshot &&
    typeof snapshot.productId === 'string' &&
    typeof snapshot.name === 'string' &&
    typeof snapshot.sellingPrice === 'number' &&
    typeof snapshot.shownPrice === 'number' &&
    typeof snapshot.currency === 'string' &&
    typeof snapshot.stock === 'number'
  )
}

function isLines(value: unknown): value is LocalLine[] {
  return Array.isArray(value) && value.every(isLine)
}

function persist(lines: LocalLine[]): LocalLine[] {
  writeStorage(storageKey, JSON.stringify(lines))
  return lines
}

export function capFor(stock: number): number {
  return Math.max(0, Math.min(maxQuantityPerItem, Math.floor(stock)))
}

export const useLocalCartStore = create<LocalCartState>((set, get) => ({
  lines: readJSON(storageKey, isLines) ?? [],
  add: (variantId, snapshot, quantity) => {
    const lines = get().lines
    const cap = capFor(snapshot.stock)
    if (cap === 0) return 'out_of_stock'
    const existing = lines.find((line) => line.variantId === variantId)
    if (!existing && lines.length >= maxCartItems) return 'full'
    const wanted = (existing?.quantity ?? 0) + quantity
    const next = Math.min(wanted, cap)
    const updated = existing
      ? lines.map((line) =>
          line.variantId === variantId ? { ...line, quantity: next, snapshot } : line,
        )
      : [...lines, { variantId, quantity: next, addedAt: new Date().toISOString(), snapshot }]
    set({ lines: persist(updated) })
    return next < wanted ? 'capped' : 'added'
  },
  setQuantity: (variantId, quantity) => {
    const lines = get().lines
    const updated =
      quantity <= 0
        ? lines.filter((line) => line.variantId !== variantId)
        : lines.map((line) =>
            line.variantId === variantId
              ? { ...line, quantity: Math.min(quantity, capFor(line.snapshot.stock)) }
              : line,
          )
    set({ lines: persist(updated) })
  },
  remove: (variantId) =>
    set({ lines: persist(get().lines.filter((line) => line.variantId !== variantId)) }),
  clear: () => set({ lines: persist([]) }),
}))
