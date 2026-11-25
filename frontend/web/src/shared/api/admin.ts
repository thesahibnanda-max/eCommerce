import { request, requestBlob } from './client'
import type {
  AdminCart,
  AdminImage,
  AdminOverview,
  AdminProduct,
  AdminUser,
  AdminUserDetail,
  ImageInfo,
  Order,
  OrderPage,
  OrderPayment,
  PaymentPage,
  Product,
  ProductInput,
  ProductUpdateInput,
} from './types'

export function createProduct(adminKey: string, product: ProductInput): Promise<Product> {
  return request<{ product: Product }>('/admin/products', {
    method: 'POST',
    adminKey,
    body: product,
  }).then((data) => data.product)
}

export function createProducts(adminKey: string, products: ProductInput[]): Promise<Product[]> {
  return request<{ products: Product[] }>('/admin/products/batch', {
    method: 'POST',
    adminKey,
    body: { products },
  }).then((data) => data.products)
}

export function updateProduct(adminKey: string, product: ProductUpdateInput): Promise<Product> {
  const { id, ...rest } = product
  return request<{ product: Product }>(`/admin/products/${encodeURIComponent(id)}`, {
    method: 'PUT',
    adminKey,
    body: rest,
  }).then((data) => data.product)
}

export function updateProducts(
  adminKey: string,
  products: ProductUpdateInput[],
): Promise<Product[]> {
  return request<{ products: Product[] }>('/admin/products/batch', {
    method: 'PUT',
    adminKey,
    body: { products },
  }).then((data) => data.products)
}

export function uploadImage(
  adminKey: string,
  file: File,
  metadata: Record<string, unknown> | null,
): Promise<ImageInfo> {
  const form = new FormData()
  form.append('file', file)
  if (metadata) form.append('metadata', JSON.stringify(metadata))
  return request<{ image: ImageInfo }>('/admin/images', {
    method: 'POST',
    adminKey,
    body: form,
  }).then((data) => data.image)
}

export function uploadImages(adminKey: string, files: File[]): Promise<ImageInfo[]> {
  const form = new FormData()
  for (const file of files) form.append('files', file)
  return request<{ images: ImageInfo[] }>('/admin/images/batch', {
    method: 'POST',
    adminKey,
    body: form,
  }).then((data) => data.images)
}

export function deleteImage(adminKey: string, id: string): Promise<string> {
  return request<{ id: string }>(`/admin/images/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    adminKey,
  }).then((data) => data.id)
}

export function deleteImages(adminKey: string, ids: string[]): Promise<string[]> {
  return request<{ ids: string[] }>('/admin/images/batch-delete', {
    method: 'POST',
    adminKey,
    body: { ids },
  }).then((data) => data.ids)
}

export function getImageInfo(id: string): Promise<ImageInfo> {
  return request<{ image: ImageInfo }>(`/images/${encodeURIComponent(id)}/info`).then(
    (data) => data.image,
  )
}

function query(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value))
  }
  const text = search.toString()
  return text ? `?${text}` : ''
}

export function adminOverview(adminKey: string, signal?: AbortSignal): Promise<AdminOverview> {
  return request<{ overview: AdminOverview }>('/admin/overview', { adminKey, signal }).then(
    (data) => data.overview,
  )
}

export function adminListUsers(
  adminKey: string,
  offset: number,
  limit: number,
  signal?: AbortSignal,
): Promise<{ users: AdminUser[]; total: number }> {
  return request(`/admin/users${query({ offset, limit })}`, { adminKey, signal })
}

export function adminGetUser(
  adminKey: string,
  id: string,
  signal?: AbortSignal,
): Promise<AdminUserDetail> {
  return request(`/admin/users/${encodeURIComponent(id)}`, { adminKey, signal })
}

export function adminListCarts(
  adminKey: string,
  offset: number,
  limit: number,
  signal?: AbortSignal,
): Promise<{ carts: AdminCart[]; total: number }> {
  return request(`/admin/carts${query({ offset, limit })}`, { adminKey, signal })
}

export function adminListProducts(
  adminKey: string,
  offset: number,
  limit: number,
  signal?: AbortSignal,
): Promise<{ products: AdminProduct[]; total: number }> {
  return request(`/admin/products${query({ offset, limit })}`, { adminKey, signal })
}

export function adminListImages(
  adminKey: string,
  offset: number,
  limit: number,
  signal?: AbortSignal,
): Promise<{ images: AdminImage[]; total: number }> {
  return request(`/admin/images${query({ offset, limit })}`, { adminKey, signal })
}

export function adminListOrders(
  adminKey: string,
  states: string[],
  cursor: string,
  signal?: AbortSignal,
): Promise<OrderPage> {
  return request(`/admin/orders${query({ states: states.join(','), cursor, limit: 25 })}`, {
    adminKey,
    signal,
  })
}

export function adminGetOrder(adminKey: string, id: string, signal?: AbortSignal): Promise<Order> {
  return request<{ order: Order }>(`/admin/orders/${encodeURIComponent(id)}`, {
    adminKey,
    signal,
  }).then((data) => data.order)
}

export function adminTransitionOrder(
  adminKey: string,
  id: string,
  to: string,
  reason: string,
): Promise<Order> {
  return request<{ order: Order }>(`/admin/orders/${encodeURIComponent(id)}/transition`, {
    method: 'POST',
    adminKey,
    body: { to, reason },
  }).then((data) => data.order)
}

export function adminListPayments(
  adminKey: string,
  states: string[],
  cursor: string,
  signal?: AbortSignal,
): Promise<PaymentPage> {
  return request(`/admin/payments${query({ states: states.join(','), cursor, limit: 25 })}`, {
    adminKey,
    signal,
  })
}

export function adminGetPayment(
  adminKey: string,
  id: string,
  signal?: AbortSignal,
): Promise<OrderPayment> {
  return request<{ payment: OrderPayment }>(`/admin/payments/${encodeURIComponent(id)}`, {
    adminKey,
    signal,
  }).then((data) => data.payment)
}

export function adminPaymentProof(
  adminKey: string,
  id: string,
  signal?: AbortSignal,
): Promise<Blob> {
  return requestBlob(`/admin/payments/${encodeURIComponent(id)}/proof`, { adminKey, signal })
}

export function adminDecidePayment(
  adminKey: string,
  id: string,
  status: 'SUCCEEDED' | 'FAILED',
  note: string,
): Promise<OrderPayment> {
  return request<{ payment: OrderPayment }>(`/admin/payments/${encodeURIComponent(id)}/decision`, {
    method: 'POST',
    adminKey,
    body: { status, note },
  }).then((data) => data.payment)
}

export function adminSetTracking(
  adminKey: string,
  id: string,
  tracking: { courier: string; number: string; url: string },
): Promise<Order> {
  return request<{ order: Order }>(`/admin/orders/${encodeURIComponent(id)}/tracking`, {
    method: 'POST',
    adminKey,
    body: tracking,
  }).then((data) => data.order)
}
