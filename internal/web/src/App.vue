<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useBrowseStore } from '@/stores/browse'

const store = useBrowseStore()
const router = useRouter()

onMounted(async () => {
  await store.loadCars()
  store.loadStatus()
  // Refresh live status every 60s.
  setInterval(() => store.loadStatus(), 60_000)
})

const carLabel = computed(() => {
  const car = store.cars.find((c) => c.key === store.selectedCarKey)
  return car?.display_name ?? car?.key ?? store.selectedCarKey
})

function onCarChange(e: Event) {
  const key = (e.target as HTMLSelectElement).value
  store.setCar(key)
  // Sync URL
  const q = { ...router.currentRoute.value.query, car: key }
  router.push({ query: q })
}
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <span class="app-title" @click="router.push('/')">dashmap</span>

      <div class="header-controls">
        <div v-if="store.cars.length > 1" class="car-select-wrap">
          <select :value="store.selectedCarKey" @change="onCarChange" class="car-select">
            <option v-for="car in store.cars" :key="car.key" :value="car.key">
              {{ car.display_name ?? car.key }}
            </option>
          </select>
        </div>
        <div v-else-if="carLabel" class="car-label">{{ carLabel }}</div>

        <div class="live-dot" :class="{ live: store.isLive }" :title="store.isLive ? 'Live — recent ingest' : 'No recent ingest'">
          <span class="dot" />
          <span class="live-text">{{ store.isLive ? 'Live' : 'Stale' }}</span>
        </div>
      </div>
    </header>

    <RouterView class="app-content" />
  </div>
</template>

<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --sidebar-w: 340px;
  --header-h: 44px;
  --bg-app: #0e0f11;
  --bg-sidebar: #13141a;
  --bg-surface: #1a1c23;
  --bg-hover: #20222b;
  --bg-selected: #1b2a45;
  --border: rgba(255,255,255,0.07);
  --text-1: #e2e4ea;
  --text-2: #9199a6;
  --text-3: #555d6a;
  --accent: #4f8ef7;
  --accent-muted: rgba(79,142,247,0.14);
  --kind-sentry: #f59e0b;
  --kind-saved: #60a5fa;
  --kind-recent: #94a3b8;
  --radius: 6px;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 13px;
  color: var(--text-1);
}

html, body, #app {
  height: 100%;
  background: var(--bg-app);
  overflow: hidden;
}
</style>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.app-header {
  height: var(--header-h);
  background: var(--bg-sidebar);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 16px;
  flex-shrink: 0;
  z-index: 10;
}

.app-title {
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.3px;
  color: var(--text-1);
  cursor: pointer;
  user-select: none;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
}

.car-select {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  color: var(--text-1);
  border-radius: var(--radius);
  padding: 3px 8px;
  font-size: 12px;
  cursor: pointer;
}

.car-label {
  font-size: 12px;
  color: var(--text-2);
}

.live-dot {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: var(--text-3);
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--text-3);
}

.live-dot.live .dot {
  background: #34d399;
  box-shadow: 0 0 0 3px rgba(52,211,153,0.2);
}

.live-dot.live .live-text { color: #34d399; }

.app-content {
  flex: 1;
  min-height: 0;
}
</style>
