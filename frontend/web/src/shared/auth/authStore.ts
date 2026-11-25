import { create } from 'zustand'
import { configureTokens } from '@/shared/api/client'
import type { Session } from '@/shared/api/types'
import { readJSON, removeStorage, writeStorage } from '@/shared/lib/safeStorage'

const storageKey = 'ai.session'

type StoredSession = { refreshToken: string; userId: string; contact: string }

type AuthState = {
  accessToken: string | null
  refreshToken: string | null
  userId: string | null
  contact: string | null
  restored: boolean
  setSession: (session: Session, contact?: string) => void
  markRestored: () => void
  clear: () => void
}

function isStoredSession(value: unknown): value is StoredSession {
  if (!value || typeof value !== 'object') return false
  const record = value as Record<string, unknown>
  return (
    typeof record.refreshToken === 'string' &&
    typeof record.userId === 'string' &&
    typeof record.contact === 'string'
  )
}

const stored = readJSON(storageKey, isStoredSession)

export const useAuthStore = create<AuthState>((set, get) => ({
  accessToken: null,
  refreshToken: stored?.refreshToken ?? null,
  userId: stored?.userId ?? null,
  contact: stored?.contact ?? null,
  restored: stored === null,
  setSession: (session, contact) => {
    const nextContact = contact ?? get().contact ?? ''
    writeStorage(
      storageKey,
      JSON.stringify({
        refreshToken: session.refresh_token,
        userId: session.id,
        contact: nextContact,
      }),
    )
    set({
      accessToken: session.access_token,
      refreshToken: session.refresh_token,
      userId: session.id,
      contact: nextContact,
      restored: true,
    })
  },
  markRestored: () => set({ restored: true }),
  clear: () => {
    removeStorage(storageKey)
    set({ accessToken: null, refreshToken: null, userId: null, contact: null, restored: true })
  },
}))

configureTokens({
  accessToken: () => useAuthStore.getState().accessToken,
  refreshToken: () => useAuthStore.getState().refreshToken,
  onRefreshed: (session) => useAuthStore.getState().setSession(session),
  onExpired: () => useAuthStore.getState().clear(),
})

export function useIsSignedIn(): boolean {
  return useAuthStore((state) => state.refreshToken !== null)
}
