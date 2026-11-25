import type { QueryClient } from '@tanstack/react-query'
import { mergeCart } from '@/shared/api/cart'
import { refreshSession } from '@/shared/api/client'
import { queryKeys } from '@/shared/api/queries'
import type { Session } from '@/shared/api/types'
import { useLocalCartStore } from '@/shared/cart/localCartStore'
import { toast } from '@/shared/lib/toastStore'
import { useAuthStore } from './authStore'

export async function completeSignIn(
  client: QueryClient,
  session: Session,
  contact: string,
): Promise<void> {
  useAuthStore.getState().setSession(session, contact)
  const lines = useLocalCartStore.getState().lines
  if (lines.length === 0) {
    await client.invalidateQueries({ queryKey: queryKeys.cart })
    return
  }
  try {
    const result = await mergeCart(
      lines.map((line) => ({ variant_id: line.variantId, quantity: line.quantity })),
    )
    client.setQueryData(queryKeys.cart, result.cart)
    useLocalCartStore.getState().clear()
    if (result.skipped.length > 0)
      toast(`${result.skipped.length} item(s) were unavailable and left out.`, 'info')
  } catch {
    toast('We could not move your bag items. They are still saved on this device.', 'error')
    await client.invalidateQueries({ queryKey: queryKeys.cart })
  }
}

export function signOut(client: QueryClient): void {
  useAuthStore.getState().clear()
  for (const key of [
    queryKeys.cart,
    queryKeys.me,
    queryKeys.orders,
    queryKeys.addresses,
    queryKeys.quote,
  ]) {
    client.removeQueries({ queryKey: key })
  }
  client.removeQueries({ queryKey: ['order'] })
}

export async function restoreSession(): Promise<void> {
  const state = useAuthStore.getState()
  if (state.restored) return
  await refreshSession()
  useAuthStore.getState().markRestored()
}
