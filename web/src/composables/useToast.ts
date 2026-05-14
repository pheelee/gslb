import { ref } from 'vue'

export type ToastVariant = 'success' | 'error' | 'warning' | 'info'

interface Toast {
  id: string
  message: string
  variant: ToastVariant
  duration: number
}

const toasts = ref<Toast[]>([])

let idCounter = 0

export function useToast() {
  const showToast = (message: string, variant: ToastVariant = 'info', duration: number = 3000) => {
    const id = `toast-${++idCounter}`
    const toast: Toast = { id, message, variant, duration }
    toasts.value.push(toast)

    // Auto-remove after duration
    setTimeout(() => {
      removeToast(id)
    }, duration)

    return id
  }

  const removeToast = (id: string) => {
    const index = toasts.value.findIndex(t => t.id === id)
    if (index > -1) {
      toasts.value.splice(index, 1)
    }
  }

  return {
    toasts,
    showToast,
    removeToast
  }
}

// Global toast instance for use outside components
const globalToasts = useToast()

export const toast = {
  success: (message: string, duration?: number) => globalToasts.showToast(message, 'success', duration),
  error: (message: string, duration?: number) => globalToasts.showToast(message, 'error', duration),
  warning: (message: string, duration?: number) => globalToasts.showToast(message, 'warning', duration),
  info: (message: string, duration?: number) => globalToasts.showToast(message, 'info', duration),
  toasts: globalToasts.toasts,
  remove: globalToasts.removeToast
}
