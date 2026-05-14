<script setup lang="ts">
import { RouterView } from 'vue-router'
import Modal from '@/components/Modal.vue'
import Toast from '@/components/Toast.vue'
</script>

<template>
  <div id="app">
    <RouterView />
    <Modal />
    <Toast />
  </div>
</template>
