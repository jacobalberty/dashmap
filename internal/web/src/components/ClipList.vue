<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useBrowseStore } from '@/stores/browse'
import { clipLocalDate, formatLocalTime, formatDateHeader } from '@/lib/datetime'
import { Temporal } from 'temporal-polyfill'

const store = useBrowseStore()
const router = useRouter()

// Group clips by local date.
const grouped = computed(() => {
  const groups = new Map<string, { label: string; clips: typeof store.clips }>()
  for (const clip of store.clips) {
    const d = clipLocalDate(clip.clip_ts, store.tz)
    const key = d.toString()
    if (!groups.has(key)) groups.set(key, { label: formatDateHeader(d), clips: [] })
    groups.get(key)!.clips.push(clip)
  }
  // Return in newest-first order (map insertion order reflects API newest-first).
  return Array.from(groups.values())
})

// IntersectionObserver sentinel for infinite scroll.
const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      if (entries[0].isIntersecting) store.loadMoreClips()
    },
    { threshold: 0.1 },
  )
  if (sentinel.value) observer.observe(sentinel.value)
})

onUnmounted(() => {
  observer?.disconnect()
})

function onRowClick(id: number) {
  store.selectClip(store.selectedClipId === id ? null : id)
}

function onRowDblClick(id: number) {
  store.selectClip(id)
  router.push(`/clip/${id}`)
}

function onRowKeydown(e: KeyboardEvent, id: number) {
  if (e.key === 'Enter') {
    router.push(`/clip/${id}`)
  } else if (e.key === 'Escape') {
    store.selectClip(null)
  }
}
</script>

<template>
  <div class="clip-list">
    <div v-if="!store.hasRange" class="empty-state">
      Pick a date range on the calendar to see clips.
    </div>

    <div v-else-if="store.clipsLoading && store.clips.length === 0" class="empty-state">
      Loading…
    </div>

    <div v-else-if="store.clips.length === 0 && !store.clipsLoading" class="empty-state">
      No clips in this range.
    </div>

    <template v-else>
      <div v-for="group in grouped" :key="group.label">
        <div class="date-header">{{ group.label }}</div>
        <div
          v-for="clip in group.clips"
          :key="clip.id"
          :class="['clip-row', { selected: store.selectedClipId === clip.id }]"
          tabindex="0"
          @click="onRowClick(clip.id)"
          @dblclick="onRowDblClick(clip.id)"
          @keydown="onRowKeydown($event, clip.id)"
        >
          <div class="thumb-wrap">
            <img v-if="clip.thumb_url" :src="clip.thumb_url" class="thumb" loading="lazy" />
            <div v-else :class="['thumb-placeholder', `kind-bg-${clip.kind}`]">
              {{ clip.kind[0].toUpperCase() }}
            </div>
          </div>

          <div class="clip-info">
            <div class="clip-top">
              <span class="clip-time">{{ formatLocalTime(clip.clip_ts, store.tz) }}</span>
              <span :class="['kind-badge', `kind-${clip.kind}`]">{{ clip.kind }}</span>
              <span v-if="!clip.lat" class="no-gps">no GPS</span>
            </div>
            <div v-if="clip.reason" class="clip-reason">{{ clip.reason }}</div>
            <div v-if="clip.city" class="clip-city">{{ clip.city }}</div>
          </div>

          <button class="play-btn" title="Play" @click.stop="router.push(`/clip/${clip.id}`)">
            ▶
          </button>
        </div>
      </div>

      <div ref="sentinel" class="sentinel" />
      <div v-if="store.clipsLoading" class="load-more-spinner">Loading…</div>
    </template>
  </div>
</template>

<style scoped>
.clip-list {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.empty-state {
  padding: 32px 16px;
  text-align: center;
  color: var(--text-3);
  font-size: 12px;
}

.date-header {
  position: sticky;
  top: 0;
  background: var(--bg-sidebar);
  padding: 5px 12px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.6px;
  color: var(--text-3);
  border-bottom: 1px solid var(--border);
  z-index: 1;
}

.clip-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  outline: none;
  transition: background 0.08s;
}

.clip-row:hover { background: var(--bg-hover); }
.clip-row.selected { background: var(--bg-selected); }
.clip-row:focus-visible { box-shadow: inset 0 0 0 2px var(--accent); }

.thumb-wrap { flex-shrink: 0; }

.thumb {
  width: 52px;
  height: 36px;
  object-fit: cover;
  border-radius: 3px;
  display: block;
  background: var(--bg-surface);
}

.thumb-placeholder {
  width: 52px;
  height: 36px;
  border-radius: 3px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 700;
  color: rgba(255,255,255,0.5);
}

.kind-bg-sentry { background: rgba(245,158,11,0.15); }
.kind-bg-saved  { background: rgba(96,165,250,0.12); }
.kind-bg-recent { background: rgba(148,163,184,0.08); }

.clip-info { flex: 1; min-width: 0; }

.clip-top {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 2px;
}

.clip-time {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-1);
  font-variant-numeric: tabular-nums;
}

.kind-badge {
  display: inline-block;
  padding: 0 5px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.4px;
}

.kind-sentry { background: rgba(245,158,11,0.12); color: var(--kind-sentry); }
.kind-saved  { background: rgba(96,165,250,0.12); color: var(--kind-saved); }
.kind-recent { background: rgba(148,163,184,0.08); color: var(--kind-recent); }

.no-gps {
  font-size: 10px;
  color: var(--text-3);
  margin-left: auto;
}

.clip-reason {
  font-size: 11px;
  color: var(--text-2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.clip-city {
  font-size: 10px;
  color: var(--text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.play-btn {
  flex-shrink: 0;
  background: none;
  border: none;
  color: var(--text-3);
  font-size: 10px;
  cursor: pointer;
  padding: 4px;
  border-radius: 3px;
  opacity: 0;
  transition: opacity 0.1s;
}

.clip-row:hover .play-btn,
.clip-row.selected .play-btn { opacity: 1; }
.play-btn:hover { color: var(--accent); background: var(--accent-muted); }

.sentinel { height: 1px; }

.load-more-spinner {
  text-align: center;
  padding: 10px;
  font-size: 11px;
  color: var(--text-3);
}
</style>
