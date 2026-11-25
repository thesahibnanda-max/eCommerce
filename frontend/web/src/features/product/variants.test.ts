import type { Product, Variant } from '@/shared/api/types'
import {
  findVariant,
  initialSelection,
  isOptionAvailable,
  optionGroups,
  selectOption,
} from './variants'

function variant(
  id: string,
  attributes: Record<string, string>,
  stock: number,
  price = 100,
): Variant {
  return {
    id,
    sku: id,
    attributes,
    shown_price: price,
    selling_price: price,
    currency: 'INR',
    stock_units: stock,
  }
}

const product: Product = {
  id: 'p',
  name: 'Kurta',
  description: '',
  created_at: '',
  updated_at: '',
  variants: [
    variant('red-m', { color: 'Red', size: 'M' }, 2, 900),
    variant('red-l', { color: 'Red', size: 'L' }, 0, 900),
    variant('blue-m', { color: 'Blue', size: 'M' }, 5, 800),
  ],
}

describe('variants', () => {
  it('groups options in first-seen order', () => {
    expect(optionGroups(product)).toEqual([
      { name: 'color', values: ['Red', 'Blue'] },
      { name: 'size', values: ['M', 'L'] },
    ])
  })

  it('finds the exact variant only when every group is chosen', () => {
    expect(findVariant(product, { color: 'Red', size: 'M' })?.id).toBe('red-m')
    expect(findVariant(product, { color: 'Red' })).toBeUndefined()
  })

  it('marks out of stock combinations unavailable', () => {
    expect(isOptionAvailable(product, { color: 'Red', size: 'M' }, 'size', 'L')).toBe(false)
    expect(isOptionAvailable(product, { color: 'Red', size: 'M' }, 'color', 'Blue')).toBe(true)
  })

  it('falls back to a real combination when switching to an impossible one', () => {
    expect(selectOption(product, { color: 'Red', size: 'L' }, 'color', 'Blue')).toEqual({
      color: 'Blue',
      size: 'M',
    })
  })

  it('starts on the cheapest in-stock variant', () => {
    expect(initialSelection(product)).toEqual({ color: 'Blue', size: 'M' })
  })
})
