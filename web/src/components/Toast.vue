<template>
  <Teleport to="body">
    <div class="toast-container">
      <TransitionGroup name="toast">
        <div v-for="toast in toasts" :key="toast.id" class="toast" :class="`toast-${toast.variant}`" role="alert">
          <span class="toast-message">{{ toast.message }}</span>
          <button class="toast-close" @click="remove(toast.id)" aria-label="Close notification">
            ×
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { toast } from '@/composables/useToast'

const { toasts, remove } = toast
</script>

<style scoped>
.toast-container {
  position: fixed;
  top: 1rem;
  right: 1rem;
  z-index: 10000;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  pointer-events: none;
}

.toast {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.875rem 1rem;
  border-radius: 8px;
  min-width: 300px;
  max-width: 400px;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
  pointer-events: auto;
  border: 1px solid transparent;
}

.toast-success {
  background: rgba(166, 227, 161, 0.15);
  color: var(--ctp-green);
  border-color: rgba(166, 227, 161, 0.3);
}

.toast-error {
  background: rgba(243, 139, 168, 0.15);
  color: var(--ctp-red);
  border-color: rgba(243, 139, 168, 0.3);
}

.toast-warning {
  background: rgba(249, 226, 175, 0.15);
  color: var(--ctp-yellow);
  border-color: rgba(249, 226, 175, 0.3);
}

.toast-info {
  background: rgba(137, 180, 250, 0.15);
  color: var(--ctp-blue);
  border-color: rgba(137, 180, 250, 0.3);
}

.toast-message {
  font-size: 0.875rem;
  font-weight: 500;
}

.toast-close {
  background: none;
  border: none;
  color: inherit;
  font-size: 1.25rem;
  cursor: pointer;
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  opacity: 0.7;
  transition: opacity 0.2s;
}

.toast-close:hover {
  opacity: 1;
  background: rgba(255, 255, 255, 0.1);
}

/* Transitions */
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translateX(100%);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(100%);
}

@media (max-width: 640px) {
  .toast-container {
    left: 1rem;
    right: 1rem;
  }

  .toast {
    min-width: auto;
    max-width: none;
  }
}
</style>
