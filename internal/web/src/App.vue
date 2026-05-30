<script setup lang="ts">
import { ref, onMounted } from 'vue'

const status = ref<'checking' | 'ok' | 'error'>('checking')

onMounted(async () => {
  try {
    const res = await fetch('/livez')
    status.value = res.ok ? 'ok' : 'error'
  } catch {
    status.value = 'error'
  }
})
</script>

<template>
  <main>
    <h1>dashmap</h1>
    <p>Browse Tesla dashcam footage on a map.</p>
    <p>
      API status:
      <span :class="status">{{ status }}</span>
    </p>
  </main>
</template>

<style scoped>
.ok { color: green; }
.error { color: red; }
.checking { color: gray; }
</style>
