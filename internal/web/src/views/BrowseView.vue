<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Temporal } from 'temporal-polyfill'
import { useBrowseStore } from '@/stores/browse'
import CalendarHeatmap from '@/components/CalendarHeatmap.vue'
import ClipList from '@/components/ClipList.vue'
import TrackMap from '@/components/TrackMap.vue'

const store = useBrowseStore()
const route = useRoute()
const router = useRouter()

onMounted(async () => {
  // Hydrate state from URL query params.
  const { from, to, car, sel } = route.query

  if (car && typeof car === 'string' && car !== store.selectedCarKey) {
    store.selectedCarKey = car
  }

  // Load calendar for the current month (cars already loaded by App.vue).
  await store.loadCalendar()

  if (from && to && typeof from === 'string' && typeof to === 'string') {
    try {
      const f = Temporal.PlainDate.from(from)
      const t = Temporal.PlainDate.from(to)
      await store.applyRange(f, t)
      if (sel && typeof sel === 'string') {
        const id = Number(sel)
        if (id) store.selectClip(id)
      }
    } catch {
      // Ignore malformed URL params.
    }
  }
})

// Sync range to URL (push = adds history entry).
watch(
  [() => store.rangeFrom, () => store.rangeTo],
  ([from, to]) => {
    if (!from || !to) return
    const q: Record<string, string> = { ...flatQuery(route.query) }
    q.from = from.toString()
    q.to = to.toString()
    delete q.sel
    router.push({ query: q })
  },
)

// Sync selection to URL (replace = no history spam).
watch(() => store.selectedClipId, (id) => {
  const q = { ...flatQuery(route.query) }
  if (id != null) {
    q.sel = String(id)
  } else {
    delete q.sel
  }
  router.replace({ query: q })
})

// Sync car to URL.
watch(() => store.selectedCarKey, (key) => {
  if (!key) return
  const q = { ...flatQuery(route.query), car: key }
  router.replace({ query: q })
})

function flatQuery(q: typeof route.query): Record<string, string> {
  // Map avoids writing user-controlled keys as object properties
  // (prototype pollution / remote property injection).
  const out = new Map<string, string>()
  for (const [k, v] of Object.entries(q)) {
    if (typeof v === 'string') out.set(k, v)
  }
  return Object.fromEntries(out)
}
</script>

<template>
  <div class="browse">
    <aside class="sidebar">
      <CalendarHeatmap />
      <ClipList />
    </aside>
    <main class="map-area">
      <TrackMap />
    </main>
  </div>
</template>

<style scoped>
.browse {
  display: flex;
  height: 100%;
  overflow: hidden;
}

.sidebar {
  width: var(--sidebar-w);
  flex-shrink: 0;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.map-area {
  flex: 1;
  min-width: 0;
  position: relative;
}
</style>
