import { activeFilterCount, fromURL, toQuery, toURL } from './searchParams'

describe('search params', () => {
  it('round trips through the URL', () => {
    const params = fromURL(
      new URLSearchParams('q=silk&cat=sarees,kurtas&min=500&max=2000&stock=1&sort=price_asc'),
    )
    expect(params).toEqual({
      text: 'silk',
      categories: ['sarees', 'kurtas'],
      minPrice: 500,
      maxPrice: 2000,
      inStockOnly: true,
      sort: 'price_asc',
    })
    expect(fromURL(toURL(params))).toEqual(params)
    expect(activeFilterCount(params)).toBe(6)
  })

  it('ignores bad values', () => {
    const params = fromURL(new URLSearchParams('min=-4&max=abc&sort=cheap'))
    expect(params.minPrice).toBeNull()
    expect(params.maxPrice).toBeNull()
    expect(params.sort).toBe('relevance')
  })

  it('sends whole rupees to the API', () => {
    const query = toQuery(fromURL(new URLSearchParams('min=10.5&max=20.2')))
    expect(query.minPrice).toBe(10)
    expect(query.maxPrice).toBe(21)
  })
})
