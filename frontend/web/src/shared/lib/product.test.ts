import { describe, expect, it } from 'vitest'
import type { Product } from '@/shared/api/types'
import { toneAt } from '@/shared/ui/doodles/palette'
import { isNewArrival } from './product'

const day = 24 * 60 * 60 * 1000
const now = Date.parse('2026-11-13T00:00:00Z')

function product(createdAt: string): Product {
  return {
    id: 'p',
    name: 'n',
    description: '',
    variants: [],
    created_at: createdAt,
    updated_at: createdAt,
  }
}

describe('isNewArrival', () => {
  it('is true within two weeks', () => {
    expect(isNewArrival(product(new Date(now - 3 * day).toISOString()), now)).toBe(true)
  })
  it('is false after two weeks', () => {
    expect(isNewArrival(product(new Date(now - 15 * day).toISOString()), now)).toBe(false)
  })
  it('is false for future or invalid dates', () => {
    expect(isNewArrival(product(new Date(now + day).toISOString()), now)).toBe(false)
    expect(isNewArrival(product('not-a-date'), now)).toBe(false)
  })
})

describe('toneAt', () => {
  it('cycles and handles negatives', () => {
    expect(toneAt(0)).toBe('pink')
    expect(toneAt(6)).toBe('pink')
    expect(toneAt(-1)).toBe('sky')
  })
})
