import { create } from 'zustand'

export type ToastTone = 'info' | 'success' | 'error'

export type Toast = { id: number; message: string; tone: ToastTone }

type ToastState = {
  toasts: Toast[]
  push: (message: string, tone?: ToastTone) => void
  dismiss: (id: number) => void
}

let nextID = 1

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],
  push: (message, tone = 'info') => {
    const id = nextID++
    set({ toasts: [...get().toasts.slice(-2), { id, message, tone }] })
    window.setTimeout(() => get().dismiss(id), 3200)
  },
  dismiss: (id) => set({ toasts: get().toasts.filter((toast) => toast.id !== id) }),
}))

export function toast(message: string, tone: ToastTone = 'info'): void {
  useToastStore.getState().push(message, tone)
}
