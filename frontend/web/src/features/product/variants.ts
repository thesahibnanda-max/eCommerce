import type { Product, Variant } from '@/shared/api/types'
import { primaryVariant } from '@/shared/lib/product'

export type Selection = Record<string, string>

export type OptionGroup = { name: string; values: string[] }

export function optionGroups(product: Product): OptionGroup[] {
  const groups = new Map<string, string[]>()
  for (const variant of product.variants) {
    for (const [name, value] of Object.entries(variant.attributes ?? {})) {
      const values = groups.get(name) ?? []
      if (!values.includes(value)) values.push(value)
      groups.set(name, values)
    }
  }
  return [...groups.entries()].map(([name, values]) => ({ name, values }))
}

export function splitGroups(groups: OptionGroup[]): {
  choices: OptionGroup[]
  details: { name: string; value: string }[]
} {
  const choices: OptionGroup[] = []
  const details: { name: string; value: string }[] = []
  for (const group of groups) {
    if (group.values.length > 1) choices.push(group)
    else if (group.values[0] !== undefined)
      details.push({ name: group.name, value: group.values[0] })
  }
  return { choices, details }
}

function matches(variant: Variant, selection: Selection): boolean {
  const attributes = variant.attributes ?? {}
  return Object.entries(selection).every(([name, value]) => attributes[name] === value)
}

export function findVariant(product: Product, selection: Selection): Variant | undefined {
  const groups = optionGroups(product)
  if (groups.length === 0) return undefined
  if (groups.some((group) => selection[group.name] === undefined)) return undefined
  return product.variants.find((variant) => matches(variant, selection))
}

export function isOptionAvailable(
  product: Product,
  selection: Selection,
  name: string,
  value: string,
): boolean {
  const next = { ...selection, [name]: value }
  return product.variants.some((variant) => variant.stock_units > 0 && matches(variant, next))
}

export function selectOption(
  product: Product,
  selection: Selection,
  name: string,
  value: string,
): Selection {
  const next = { ...selection, [name]: value }
  if (product.variants.some((variant) => matches(variant, next))) return next
  const fallback = product.variants.find((variant) => (variant.attributes ?? {})[name] === value)
  return { ...(fallback?.attributes ?? {}), [name]: value }
}

export function initialSelection(product: Product): Selection {
  return { ...(primaryVariant(product)?.attributes ?? {}) }
}
