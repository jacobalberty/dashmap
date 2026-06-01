<script setup lang="ts">
import { computed, reactive, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useBrowseStore } from '@/stores/browse'
import { formatLocalTime } from '@/lib/datetime'
import { groupClips, type EventEntry } from '@/lib/clipGroups'

const store = useBrowseStore()
const router = useRouter()

const grouped = computed(() => groupClips(store.clips, store.tz))

// Which events are expanded (by eventId). Vue reactive Set tracks add/delete.
const expandedEvents = reactive(new Set<number>())

function toggleEvent(eventId: number) {
  if (expandedEvents.has(eventId)) expandedEvents.delete(eventId)
  else expandedEvents.add(eventId)
}

// When a clip gets selected (e.g. by clicking its marker on the map), expand the
// owning event so the highlighted row is visible.
watch(
  () => store.selectedClipId,
  (id) => {
    if (id == null) return
    const clip = store.clips.find((c) => c.id === id)
    if (clip?.event_id != null) expandedEvents.add(clip.event_id)
  },
)

// Time label for a collapsed event: "10:08 – 10:16" (or a single time).
function eventTimeRange(ev: EventEntry): string {
  const start = formatLocalTime(ev.firstClip.clip_ts, store.tz)
  const end = formatLocalTime(ev.latestClip.clip_ts, store.tz)
  return start === end ? start : `${start} – ${end}`
}

// True if any of the event's clips is the current selection.
function eventHasSelected(ev: EventEntry): boolean {
  return store.selectedClipId != null && ev.clips.some((c) => c.id === store.selectedClipId)
}

// ── infinite scroll ───────────────────────────────────────────────────────────

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

onUnmounted(() => observer?.disconnect())

// ── interactions ────────────────────────────────────────────────────────────

function play(id: number) {
  store.selectClip(id)
  router.push(`/clip/${id}`)
}

function onClipClick(id: number) {
  store.selectClip(store.selectedClipId === id ? null : id)
}

function onClipKeydown(e: KeyboardEvent, id: number) {
  if (e.key === 'Enter') play(id)
  else if (e.key === 'Escape') store.selectClip(null)
}

function onEventKeydown(e: KeyboardEvent, ev: EventEntry) {
  if (e.key === 'Enter') play(ev.firstClip.id)
  else if (e.key === 'ArrowRight' && !expandedEvents.has(ev.eventId)) toggleEvent(ev.eventId)
  else if (e.key === 'ArrowLeft' && expandedEvents.has(ev.eventId)) toggleEvent(ev.eventId)
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
      <div v-for="group in grouped" :key="group.key">
        <div class="date-header">{{ group.label }}</div>

        <template v-for="entry in group.entries">
          <!-- ── Standalone clip ──────────────────────────────────────── -->
          <div
            v-if="entry.type === 'clip'"
            :key="`c${entry.clip.id}`"
            :class="['clip-row', { selected: store.selectedClipId === entry.clip.id }]"
            tabindex="0"
            @click="onClipClick(entry.clip.id)"
            @dblclick="play(entry.clip.id)"
            @keydown="onClipKeydown($event, entry.clip.id)"
          >
            <div class="thumb-wrap">
              <img v-if="entry.clip.thumb_url" :src="entry.clip.thumb_url" class="thumb" loading="lazy" />
              <div v-else :class="['thumb-placeholder', `kind-bg-${entry.clip.kind}`]">
                {{ entry.clip.kind[0].toUpperCase() }}
              </div>
            </div>
            <div class="clip-info">
              <div class="clip-top">
                <span class="clip-time">{{ formatLocalTime(entry.clip.clip_ts, store.tz) }}</span>
                <span :class="['kind-badge', `kind-${entry.clip.kind}`]">{{ entry.clip.kind }}</span>
                <span v-if="!entry.clip.lat" class="no-gps">no GPS</span>
              </div>
              <div v-if="entry.clip.reason" class="clip-reason">{{ entry.clip.reason }}</div>
              <div v-if="entry.clip.city" class="clip-city">{{ entry.clip.city }}</div>
            </div>
            <button class="play-btn" title="Play" @click.stop="play(entry.clip.id)">▶</button>
          </div>

          <!-- ── Event group ──────────────────────────────────────────── -->
          <template v-else :key="`e${entry.eventId}`">
            <div
              :class="['clip-row', 'event-row', { selected: eventHasSelected(entry) }]"
              tabindex="0"
              @click="play(entry.firstClip.id)"
              @keydown="onEventKeydown($event, entry)"
            >
              <button
                v-if="entry.count > 1"
                class="chevron"
                :class="{ open: expandedEvents.has(entry.eventId) }"
                :title="expandedEvents.has(entry.eventId) ? 'Collapse' : 'Expand clips'"
                @click.stop="toggleEvent(entry.eventId)"
              >›</button>
              <span v-else class="chevron-spacer" />

              <div class="thumb-wrap">
                <img v-if="entry.thumbUrl" :src="entry.thumbUrl" class="thumb" loading="lazy" />
                <div v-else :class="['thumb-placeholder', `kind-bg-${entry.kind}`]">
                  {{ entry.kind[0].toUpperCase() }}
                </div>
              </div>

              <div class="clip-info">
                <div class="clip-top">
                  <span class="clip-time">{{ eventTimeRange(entry) }}</span>
                  <span :class="['kind-badge', `kind-${entry.kind}`]">{{ entry.kind }}</span>
                  <span v-if="entry.count > 1" class="clip-count">{{ entry.count }} clips</span>
                  <span v-if="!entry.hasGps" class="no-gps">no GPS</span>
                </div>
                <div v-if="entry.reason" class="clip-reason">{{ entry.reason }}</div>
              </div>

              <button class="play-btn" title="Play event" @click.stop="play(entry.firstClip.id)">▶</button>
            </div>

            <!-- expanded minute-clips -->
            <template v-if="expandedEvents.has(entry.eventId)">
              <div
                v-for="clip in entry.clips"
                :key="`ec${clip.id}`"
                :class="['clip-row', 'sub-row', { selected: store.selectedClipId === clip.id }]"
                tabindex="0"
                @click="onClipClick(clip.id)"
                @dblclick="play(clip.id)"
                @keydown="onClipKeydown($event, clip.id)"
              >
                <span class="sub-rail" />
                <span class="clip-time sub-time">{{ formatLocalTime(clip.clip_ts, store.tz) }}</span>
                <span v-if="!clip.lat" class="no-gps">no GPS</span>
                <button class="play-btn" title="Play" @click.stop="play(clip.id)">▶</button>
              </div>
            </template>
          </template>
        </template>
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

.chevron {
  flex-shrink: 0;
  width: 16px;
  background: none;
  border: none;
  color: var(--text-3);
  font-size: 14px;
  cursor: pointer;
  padding: 0;
  transition: transform 0.12s, color 0.1s;
  line-height: 1;
}
.chevron.open { transform: rotate(90deg); }
.chevron:hover { color: var(--text-1); }
.chevron-spacer { width: 16px; flex-shrink: 0; }

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

.clip-count {
  font-size: 10px;
  color: var(--text-3);
  background: var(--bg-surface);
  border-radius: 3px;
  padding: 0 5px;
}

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

/* expanded minute-clip rows */
.sub-row {
  padding: 3px 12px 3px 0;
  gap: 6px;
}
.sub-rail {
  width: 2px;
  align-self: stretch;
  margin-left: 33px;
  background: var(--border);
  flex-shrink: 0;
}
.sub-time {
  font-size: 11px;
  color: var(--text-2);
  font-weight: 400;
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
