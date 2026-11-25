import { request } from './client'
import type {
  CategoryCount,
  Product,
  ProductPage,
  SearchParams,
  SearchResult,
  Variant,
} from './types'

export const pageSize = 20

function query(params: Record<string, string | number | boolean | null | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === null || value === undefined || value === '' || value === false) continue
    search.set(key, String(value))
  }
  const text = search.toString()
  return text ? `?${text}` : ''
}

export function listProducts(offset: number, signal?: AbortSignal): Promise<ProductPage> {
  return request<ProductPage>(
    `/products${query({ offset, limit: pageSize })}`,
    signal ? { signal } : {},
  )
}

export function listCategoryProducts(
  category: string,
  offset: number,
  signal?: AbortSignal,
): Promise<ProductPage> {
  return request<ProductPage>(
    `/products/categories/${encodeURIComponent(category)}${query({ offset, limit: pageSize })}`,
    signal ? { signal } : {},
  )
}

export function listCategories(signal?: AbortSignal): Promise<CategoryCount[]> {
  return request<{ categories: CategoryCount[] }>(
    '/products/categories',
    signal ? { signal } : {},
  ).then((data) => data.categories)
}

export function searchProducts(
  params: SearchParams,
  offset: number,
  signal?: AbortSignal,
): Promise<SearchResult> {
  const path = `/products/search${query({
    text: params.text.trim(),
    categories: params.categories.join(','),
    min_price: params.minPrice,
    max_price: params.maxPrice,
    in_stock_only: params.inStockOnly,
    sort: params.sort === 'relevance' ? '' : params.sort,
    offset,
    limit: pageSize,
  })}`
  return request<SearchResult>(path, signal ? { signal } : {})
}

export function getProduct(id: string, signal?: AbortSignal): Promise<Product> {
  return request<{ product: Product }>(
    `/products/${encodeURIComponent(id)}`,
    signal ? { signal } : {},
  ).then((data) => data.product)
}

export function getProductByVariant(
  variantID: string,
): Promise<{ product: Product; variant: Variant }> {
  return request(`/products/variants/${encodeURIComponent(variantID)}`)
}

export function recommendProducts(id: string, signal?: AbortSignal): Promise<Product[]> {
  return request<{ products: Product[] }>(
    `/products/${encodeURIComponent(id)}/recommendations${query({ limit: 10 })}`,
    signal ? { signal } : {},
  ).then((data) => data.products)
}
