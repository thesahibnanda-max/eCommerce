import { useInfiniteQuery, useQuery, type QueryClient } from '@tanstack/react-query'
import {
  getProduct,
  listCategories,
  listCategoryProducts,
  listProducts,
  pageSize,
  recommendProducts,
  searchProducts,
} from './catalog'
import type { Product, ProductPage, SearchParams } from './types'

export const queryKeys = {
  products: ['products'] as const,
  categories: ['categories'] as const,
  category: (name: string) => ['category', name] as const,
  search: (params: SearchParams) => ['search', params] as const,
  product: (id: string) => ['product', id] as const,
  recommendations: (id: string) => ['recommendations', id] as const,
  cart: ['cart'] as const,
  me: ['me'] as const,
  addresses: ['addresses'] as const,
  quote: ['checkout-quote'] as const,
  orders: ['orders'] as const,
  order: (id: string) => ['order', id] as const,
}

function nextOffset(last: ProductPage, pages: ProductPage[]): number | undefined {
  const loaded = pages.length * pageSize
  return last.products.length < pageSize || loaded >= last.total ? undefined : loaded
}

export function useProductsFeed() {
  return useInfiniteQuery({
    queryKey: queryKeys.products,
    queryFn: ({ pageParam, signal }) => listProducts(pageParam, signal),
    initialPageParam: 0,
    getNextPageParam: nextOffset,
  })
}

export function useCategoryFeed(category: string) {
  return useInfiniteQuery({
    queryKey: queryKeys.category(category),
    queryFn: ({ pageParam, signal }) => listCategoryProducts(category, pageParam, signal),
    initialPageParam: 0,
    getNextPageParam: nextOffset,
    enabled: category.length > 0,
  })
}

export function useSearchFeed(params: SearchParams) {
  return useInfiniteQuery({
    queryKey: queryKeys.search(params),
    queryFn: ({ pageParam, signal }) => searchProducts(params, pageParam, signal),
    initialPageParam: 0,
    getNextPageParam: nextOffset,
    placeholderData: (previous) => previous,
  })
}

export function useCategories() {
  return useQuery({
    queryKey: queryKeys.categories,
    queryFn: ({ signal }) => listCategories(signal),
    staleTime: 5 * 60_000,
  })
}

export function useProduct(id: string, placeholder?: Product) {
  return useQuery<Product>({
    queryKey: queryKeys.product(id),
    queryFn: ({ signal }) => getProduct(id, signal),
    enabled: id.length > 0,
    placeholderData: placeholder && placeholder.id === id ? placeholder : undefined,
  })
}

export function useRecommendations(id: string) {
  return useQuery({
    queryKey: queryKeys.recommendations(id),
    queryFn: ({ signal }) => recommendProducts(id, signal),
    enabled: id.length > 0,
    staleTime: 5 * 60_000,
  })
}

export function prefetchProduct(client: QueryClient, id: string): void {
  void client.prefetchQuery({
    queryKey: queryKeys.product(id),
    queryFn: ({ signal }) => getProduct(id, signal),
    staleTime: 60_000,
  })
}
