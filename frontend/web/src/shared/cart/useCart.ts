import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo } from 'react'
import {
  addCartItem,
  clearCart,
  fetchCart,
  removeCartItem,
  setCartItemQuantity,
} from '@/shared/api/cart'
import { errorMessage } from '@/shared/api/errors'
import { queryKeys } from '@/shared/api/queries'
import type { Cart, Product, Variant } from '@/shared/api/types'
import { useIsSignedIn } from '@/shared/auth/authStore'
import { toast } from '@/shared/lib/toastStore'
import { emptyCartView, fromLocalLines, fromServerCart, type CartView } from './cartView'
import { useLocalCartStore, type LocalSnapshot } from './localCartStore'

export function snapshotOf(product: Product, variant: Variant): LocalSnapshot {
  const snapshot: LocalSnapshot = {
    productId: product.id,
    name: product.name,
    sku: variant.sku,
    shownPrice: variant.shown_price,
    sellingPrice: variant.selling_price,
    currency: variant.currency,
    stock: variant.stock_units,
  }
  const image =
    variant.image_urls?.[0] ?? product.variants.find((v) => v.image_urls?.length)?.image_urls?.[0]
  if (image) snapshot.imageUrl = image
  if (variant.attributes) snapshot.attributes = variant.attributes
  return snapshot
}

export type CartActions = {
  cart: CartView
  isLoading: boolean
  isMutating: boolean
  add: (product: Product, variant: Variant, quantity: number) => Promise<boolean>
  setQuantity: (variantId: string, quantity: number) => void
  remove: (variantId: string) => void
  clear: () => void
}

export function useCart(): CartActions {
  const signedIn = useIsSignedIn()
  const client = useQueryClient()
  const localLines = useLocalCartStore((state) => state.lines)
  const local = useLocalCartStore.getState

  const server = useQuery({
    queryKey: queryKeys.cart,
    queryFn: fetchCart,
    enabled: signedIn,
    staleTime: 30_000,
  })

  const onCart = (cart: Cart) => client.setQueryData(queryKeys.cart, cart)
  const onError = (error: unknown) => toast(errorMessage(error), 'error')

  const addMutation = useMutation({
    mutationFn: ({ variantId, quantity }: { variantId: string; quantity: number }) =>
      addCartItem({ variant_id: variantId, quantity }),
    onSuccess: onCart,
    onError,
  })
  const setMutation = useMutation({
    mutationFn: ({ variantId, quantity }: { variantId: string; quantity: number }) =>
      quantity <= 0 ? removeCartItem(variantId) : setCartItemQuantity(variantId, quantity),
    onMutate: async ({ variantId, quantity }) => {
      await client.cancelQueries({ queryKey: queryKeys.cart })
      const previous = client.getQueryData<Cart>(queryKeys.cart)
      if (previous) {
        const items = previous.items
          .map((item) => (item.variant_id === variantId ? { ...item, quantity } : item))
          .filter((item) => item.quantity > 0)
        client.setQueryData<Cart>(queryKeys.cart, { ...previous, items })
      }
      return { previous }
    },
    onError: (error, _input, context) => {
      if (context?.previous) client.setQueryData(queryKeys.cart, context.previous)
      onError(error)
    },
    onSuccess: onCart,
  })
  const clearMutation = useMutation({ mutationFn: clearCart, onSuccess: onCart, onError })

  const cart = useMemo(() => {
    if (!signedIn) return fromLocalLines(localLines)
    return server.data ? fromServerCart(server.data) : emptyCartView
  }, [signedIn, localLines, server.data])

  return {
    cart,
    isLoading: signedIn && server.isPending,
    isMutating: addMutation.isPending || setMutation.isPending || clearMutation.isPending,
    add: async (product, variant, quantity) => {
      if (!signedIn) {
        const result = local().add(variant.id, snapshotOf(product, variant), quantity)
        if (result === 'out_of_stock') toast('This item is out of stock.', 'error')
        if (result === 'full') toast('Your cart is full.', 'error')
        if (result === 'capped') toast('Added the most we can sell you.', 'info')
        return result === 'added' || result === 'capped'
      }
      try {
        await addMutation.mutateAsync({ variantId: variant.id, quantity })
        return true
      } catch {
        return false
      }
    },
    setQuantity: (variantId, quantity) => {
      if (!signedIn) {
        local().setQuantity(variantId, quantity)
        return
      }
      setMutation.mutate({ variantId, quantity })
    },
    remove: (variantId) => {
      if (!signedIn) {
        local().remove(variantId)
        return
      }
      setMutation.mutate({ variantId, quantity: 0 })
    },
    clear: () => {
      if (!signedIn) {
        local().clear()
        return
      }
      clearMutation.mutate()
    },
  }
}
