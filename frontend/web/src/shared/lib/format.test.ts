import { discountPercent, formatPrice, isWholeAmount, titleCase } from './format'

describe('format', () => {
  it('formats whole rupees with Indian grouping and no decimals', () => {
    expect(formatPrice(149999, 'INR')).toBe('₹1,49,999')
    expect(formatPrice(1499, 'INR')).toBe('₹1,499')
  })

  it('formats other and unknown currencies as whole amounts', () => {
    expect(formatPrice(19, 'USD')).toBe('$19')
    expect(formatPrice(1000, 'ZZZ')).toContain('1,000')
  })

  it('accepts only whole non-negative amounts', () => {
    expect(isWholeAmount(1499)).toBe(true)
    expect(isWholeAmount(14.99)).toBe(false)
    expect(isWholeAmount(-1)).toBe(false)
  })

  it('computes discount percent', () => {
    expect(discountPercent(2000, 1500)).toBe(25)
    expect(discountPercent(1000, 1000)).toBe(0)
    expect(discountPercent(0, 0)).toBe(0)
  })

  it('title cases categories', () => {
    expect(titleCase('home_decor-items')).toBe('Home Decor Items')
  })
})
