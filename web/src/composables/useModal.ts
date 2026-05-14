import { ref } from 'vue'

interface ModalOptions {
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  variant?: 'danger' | 'warning' | 'info'
  showCancel?: boolean
}

const isOpen = ref(false)
const options = ref<ModalOptions>({
  title: '',
  message: '',
  confirmText: 'Confirm',
  cancelText: 'Cancel',
  variant: 'info',
  showCancel: true,
})

let resolvePromise: ((value: boolean) => void) | null = null

export function useModal() {
  const showModal = (modalOptions: ModalOptions): Promise<boolean> => {
    options.value = {
      ...options.value,
      showCancel: true,
      ...modalOptions,
    }
    isOpen.value = true
    
    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  const showAlert = (modalOptions: Omit<ModalOptions, 'showCancel'>): Promise<void> => {
    options.value = {
      ...options.value,
      showCancel: false,
      confirmText: 'OK',
      ...modalOptions,
    }
    isOpen.value = true
    
    return new Promise((resolve) => {
      resolvePromise = () => {
        resolve()
      }
    })
  }

  const confirm = () => {
    isOpen.value = false
    if (resolvePromise) {
      resolvePromise(true)
      resolvePromise = null
    }
  }

  const cancel = () => {
    isOpen.value = false
    if (resolvePromise) {
      resolvePromise(false)
      resolvePromise = null
    }
  }

  return {
    isOpen,
    options,
    showModal,
    showAlert,
    confirm,
    cancel,
  }
}

export const globalModal = useModal()
