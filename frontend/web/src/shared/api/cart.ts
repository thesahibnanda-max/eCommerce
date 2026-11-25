import { request } from './client'
import type { Cart, CartItemInput } from './types'

type CartResponse = { cart: Cart }

export function fetchCart(): Promise<Cart> {
  return request<CartResponse>('/cart', { auth: true }).then((data) => data.cart)
}

export function addCartItem(input: CartItemInput): Promise<Cart> {
  return request<CartResponse>('/cart/items', { method: 'POST', auth: true, body: input }).then(
    (data) => data.cart,
  )
}

export function setCartItemQuantity(variantID: string, quantity: number): Promise<Cart> {
  return request<CartResponse>(`/cart/items/${encodeURIComponent(variantID)}`, {
    method: 'PUT',
    auth: true,
    body: { quantity },
  }).then((data) => data.cart)
}

export function removeCartItem(variantID: string): Promise<Cart> {
  return request<CartResponse>(`/cart/items/${encodeURIComponent(variantID)}`, {
    method: 'DELETE',
    auth: true,
  }).then((data) => data.cart)
}

export function clearCart(): Promise<Cart> {
  return request<CartResponse>('/cart', { method: 'DELETE', auth: true }).then((data) => data.cart)
}

export function mergeCart(items: CartItemInput[]): Promise<{ cart: Cart; skipped: string[] }> {
  return request('/cart/merge', { method: 'POST', auth: true, body: { items } })
}
