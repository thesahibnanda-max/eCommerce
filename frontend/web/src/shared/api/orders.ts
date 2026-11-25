import { request } from './client'
import type { Order, OrderPage, Quote } from './types'

type OrderResponse = { order: Order }

export function fetchQuote(signal?: AbortSignal): Promise<Quote> {
  return request<{ quote: Quote }>('/checkout/quote', { auth: true, signal }).then(
    (data) => data.quote,
  )
}

export function placeOrder(addressID: string, idempotencyKey: string): Promise<Order> {
  return request<OrderResponse>('/orders', {
    method: 'POST',
    auth: true,
    body: { address_id: addressID, idempotency_key: idempotencyKey },
  }).then((data) => data.order)
}

export function listOrders(cursor: string, signal?: AbortSignal): Promise<OrderPage> {
  const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''
  return request<OrderPage>(`/orders${query}`, { auth: true, signal })
}

export function getOrder(id: string, signal?: AbortSignal): Promise<Order> {
  return request<OrderResponse>(`/orders/${encodeURIComponent(id)}`, { auth: true, signal }).then(
    (data) => data.order,
  )
}

export function payOrder(id: string): Promise<Order> {
  return request<OrderResponse>(`/orders/${encodeURIComponent(id)}/pay`, {
    method: 'POST',
    auth: true,
  }).then((data) => data.order)
}

export function cancelOrder(id: string): Promise<Order> {
  return request<OrderResponse>(`/orders/${encodeURIComponent(id)}/cancel`, {
    method: 'POST',
    auth: true,
  }).then((data) => data.order)
}

export function submitPaymentProof(id: string, transactionID: string, file: File): Promise<Order> {
  const form = new FormData()
  form.append('transaction_id', transactionID)
  form.append('file', file)
  return request<OrderResponse>(`/orders/${encodeURIComponent(id)}/payment-proof`, {
    method: 'POST',
    auth: true,
    body: form,
  }).then((data) => data.order)
}

export function confirmProviderRedirect(
  query: string,
): Promise<{ order_id: string; payment_id: string; payment_state: string }> {
  return request(`/payments/redirect${query}`)
}
