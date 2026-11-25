import type { SearchParams, SearchSort } from '@/shared/api/types'

const sorts: SearchSort[] = ['relevance', 'price_asc', 'price_desc', 'newest']

export const sortLabels: Record<SearchSort, string> = {
  relevance: 'Best match',
  price_asc: 'Price: low to high',
  price_desc: 'Price: high to low',
  newest: 'Newest',
}

export const sortOptions = sorts

function readNumber(value: string | null): number | null {
  if (value === null || value.trim() === '') return null
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null
}

export function fromURL(params: URLSearchParams): SearchParams {
  const sort = params.get('sort')
  return {
    text: params.get('q') ?? '',
    categories: (params.get('cat') ?? '')
      .split(',')
      .map((c) => c.trim())
      .filter(Boolean),
    minPrice: readNumber(params.get('min')),
    maxPrice: readNumber(params.get('max')),
    inStockOnly: params.get('stock') === '1',
    sort: sorts.includes(sort as SearchSort) ? (sort as SearchSort) : 'relevance',
  }
}

export function toURL(params: SearchParams): URLSearchParams {
  const out = new URLSearchParams()
  if (params.text.trim()) out.set('q', params.text.trim())
  if (params.categories.length) out.set('cat', params.categories.join(','))
  if (params.minPrice !== null) out.set('min', String(params.minPrice))
  if (params.maxPrice !== null) out.set('max', String(params.maxPrice))
  if (params.inStockOnly) out.set('stock', '1')
  if (params.sort !== 'relevance') out.set('sort', params.sort)
  return out
}

export function toQuery(params: SearchParams): SearchParams {
  return {
    ...params,
    minPrice: params.minPrice === null ? null : Math.floor(params.minPrice),
    maxPrice: params.maxPrice === null ? null : Math.ceil(params.maxPrice),
  }
}

export function activeFilterCount(params: SearchParams): number {
  return (
    params.categories.length +
    (params.minPrice !== null ? 1 : 0) +
    (params.maxPrice !== null ? 1 : 0) +
    (params.inStockOnly ? 1 : 0) +
    (params.sort !== 'relevance' ? 1 : 0)
  )
}
