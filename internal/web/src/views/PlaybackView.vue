<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getClipDetail, getClipTrack } from '@/lib/api'
import { formatLocalDate, localTz } from '@/lib/datetime'
import { backendTelemetry, extendTrack } from '@/lib/playback'
import { usePlaybackSync } from '@/composables/usePlaybackSync'
import CameraGrid from '@/components/CameraGrid.vue'
import TransportBar from '@/components/TransportBar.vue'
import SeiHud from '@/components/SeiHud.vue'
import PlaybackMap from '@/components/PlaybackMap.vue'
import type { ClipDetail, ClipTrackPoint } from '@/types'

const route = useRoute()
const router = useRouter()
const tz = localTz()

// ── state ─────────────────────────────────────────────────────────────────────

const clip = ref<ClipDetail | null>(null)
const trackPts = ref<ClipTrackPoint[]>([])
const loading = ref(true)
const loadError = ref<string | null>(null)

const sync = usePlaybackSync()

// ── prefs (localStorage) ──────────────────────────────────────────────────────

interface Prefs {
  autoplayNext: boolean
  showHud: boolean
  mapExpanded: boolean
  rate: number
  speedUnit: 'mph' | 'kmh'
}

function loadPrefs(): Prefs {
  try {
    const raw = localStorage.getItem('dashmap-playback-prefs')
    return raw
      ? { autoplayNext: true, showHud: true, mapExpanded: true, rate: 1, speedUnit: 'mph', ...JSON.parse(raw) }
      : { autoplayNext: true, showHud: true, mapExpanded: true, rate: 1, speedUnit: 'mph' }
  } catch {
    return { autoplayNext: true, showHud: true, mapExpanded: true, rate: 1, speedUnit: 'mph' }
  }
}

const prefs = ref<Prefs>(loadPrefs())

watch(prefs, (p) => {
  try { localStorage.setItem('dashmap-playback-prefs', JSON.stringify(p)) } catch { /* ignore storage errors */ }
  sync.setRate(p.rate)
}, { deep: true })

// ── derived telemetry ─────────────────────────────────────────────────────────

// clipTsMs/durationSec/extended/source are reactive so they recompute when
// sync.duration arrives from loadedmetadata, filling in the stubs retroactively.
const clipTsMs = computed(() => (clip.value ? new Date(clip.value.clip_ts).getTime() : 0))
const durationSec = computed(() => sync.duration.value || (clip.value?.duration_s ?? 0))
const extended = computed(() => extendTrack(trackPts.value, clipTsMs.value, durationSec.value))
const source = computed(() => backendTelemetry(clipTsMs.value, extended.value.points))

const telemetry = computed(() => source.value.at(sync.currentTime.value))
const hasGps = computed(() => source.value.hasGps)
const hasSei = computed(() => clip.value?.has_sei ?? false)

const currentLat = computed(() => telemetry.value?.lat ?? null)
const currentLon = computed(() => telemetry.value?.lon ?? null)
const currentHeading = computed(() => telemetry.value?.headingDeg ?? null)

// ── load clip ─────────────────────────────────────────────────────────────────

const cameraGridRef = ref<InstanceType<typeof CameraGrid> | null>(null)

async function loadClip(id: number) {
  loading.value = true
  loadError.value = null
  clip.value = null
  trackPts.value = []
  sync.reset()

  try {
    const [detail, pts] = await Promise.all([getClipDetail(id), getClipTrack(id)])
    clip.value = detail
    trackPts.value = pts
    sync.setRate(prefs.value.rate)
  } catch (e) {
    loadError.value = String(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  const id = Number(route.params.id)
  if (id) loadClip(id)
  else loadError.value = 'Invalid clip ID'
})

watch(
  () => route.params.id,
  (id) => {
    const n = Number(id)
    if (n) loadClip(n)
  },
)

// ── autoplay next when ended ──────────────────────────────────────────────────

watch(sync.playing, (nowPlaying, wasPlaying) => {
  // Detect transition to stopped after playback was running.
  if (!nowPlaying && wasPlaying && clip.value && prefs.value.autoplayNext) {
    // The 'ended' event sets playing=false; check if we're actually at the end.
    const atEnd = sync.duration.value > 0 &&
      Math.abs(sync.currentTime.value - sync.duration.value) < 0.5
    if (atEnd && clip.value.next_clip_id) {
      router.push(`/clip/${clip.value.next_clip_id}`)
    }
  }
})

// ── keyboard shortcuts ────────────────────────────────────────────────────────

function onKeyDown(e: KeyboardEvent) {
  if (e.target instanceof HTMLInputElement || e.target instanceof HTMLSelectElement) return
  if (e.target instanceof HTMLTextAreaElement) return

  switch (e.key) {
    case ' ':
      e.preventDefault()
      sync.toggle()
      break
    case 'ArrowLeft':
      e.preventDefault()
      sync.seekBy(-10)
      break
    case 'ArrowRight':
      e.preventDefault()
      sync.seekBy(10)
      break
    case 'm':
    case 'M':
      if (hasSei.value) prefs.value.showHud = !prefs.value.showHud
      break
    case 'n':
    case 'N':
      if (clip.value?.next_clip_id) router.push(`/clip/${clip.value.next_clip_id}`)
      break
    default:
      if (/^[1-6]$/.test(e.key)) {
        const idx = parseInt(e.key) - 1
        if (clip.value && idx < clip.value.segments.length) {
          cameraGridRef.value?.focusIndex(idx)
        }
      }
  }
}

onMounted(() => window.addEventListener('keydown', onKeyDown))
onUnmounted(() => window.removeEventListener('keydown', onKeyDown))

// ── transport handlers ────────────────────────────────────────────────────────

function onFullscreen() {
  cameraGridRef.value?.fullscreenAll()
}

// ── map seek (drag dot/path = scrub, tap = jump) ──────────────────────────────

function onMapScrub(ll: { lng: number; lat: number }) {
  const t = source.value.timeAt(ll.lng, ll.lat)
  if (t != null) sync.scrub(t)
}

function onMapSeek(ll: { lng: number; lat: number }) {
  const t = source.value.timeAt(ll.lng, ll.lat)
  if (t != null) sync.seek(t)
}
</script>

<template>
  <div class="playback">
    <!-- Header -->
    <div class="playback-header">
      <button class="back-btn" @click="router.push('/')">← back to map</button>
      <template v-if="clip">
        <span class="clip-date">{{ formatLocalDate(clip.clip_ts, tz) }}</span>
        <span :class="['kind-badge', `kind-${clip.kind}`]">{{ clip.kind }}</span>
        <span v-if="clip.event?.reason" class="clip-reason">{{ clip.event.reason }}</span>
      </template>
      <span class="spacer" />
      <!-- prev / next navigation -->
      <template v-if="clip">
        <button
          v-if="clip.prev_clip_id"
          class="nav-btn"
          @click="router.push(`/clip/${clip.prev_clip_id}`)"
          title="Previous clip"
        >← prev</button>
        <button
          v-if="clip.next_clip_id"
          class="nav-btn"
          @click="router.push(`/clip/${clip.next_clip_id}`)"
          title="Next clip"
        >next →</button>
      </template>
    </div>

    <!-- Loading / error -->
    <div v-if="loading" class="state-msg">Loading…</div>
    <div v-else-if="loadError" class="state-msg error">{{ loadError }}</div>

    <!-- Player -->
    <template v-else-if="clip">
      <div class="player-body">
        <!-- Camera grid -->
        <div class="video-area">
          <CameraGrid
            ref="cameraGridRef"
            :segments="clip.segments"
            :register-video="sync.registerVideo"
            :get-video="sync.getVideo"
          />
          <!-- Floating map panel -->
          <div v-if="hasGps && prefs.mapExpanded" class="map-pip">
            <PlaybackMap
              :key="clip?.id"
              :track="trackPts"
              :head-stub="extended.headStub"
              :tail-stub="extended.tailStub"
              :current-lat="currentLat"
              :current-lon="currentLon"
              :current-heading="currentHeading"
              @scrub="onMapScrub"
              @seek="onMapSeek"
            />
            <button class="map-close" @click="prefs.mapExpanded = false" title="Hide map">✕</button>
          </div>
          <button
            v-if="hasGps && !prefs.mapExpanded"
            class="map-show-btn"
            @click="prefs.mapExpanded = true"
            title="Show map"
          >🗺</button>
        </div>

        <!-- Transport -->
        <TransportBar
          :playing="sync.playing.value"
          :current-time="sync.currentTime.value"
          :duration="sync.duration.value"
          :rate="sync.rate.value"
          :download-url="clip.download_url"
          @toggle="sync.toggle()"
          @scrub="sync.scrub($event)"
          @seek="sync.seek($event)"
          @seek-by="sync.seekBy($event)"
          @set-rate="(r) => { sync.setRate(r); prefs.rate = r }"
          @fullscreen="onFullscreen"
        />

        <!-- SEI HUD -->
        <SeiHud
          v-if="hasSei && prefs.showHud"
          :telemetry="telemetry"
          :speed-unit="prefs.speedUnit"
        />

        <!-- Footer options -->
        <div class="options-bar">
          <label class="opt-label" v-if="hasSei">
            <input type="checkbox" v-model="prefs.showHud" />
            show metadata
          </label>
          <label class="opt-label">
            <input type="checkbox" v-model="prefs.autoplayNext" />
            autoplay next clip
          </label>
          <label v-if="hasSei" class="opt-label unit-toggle">
            speed:
            <button
              :class="['unit-btn', { active: prefs.speedUnit === 'mph' }]"
              @click="prefs.speedUnit = 'mph'"
            >mph</button>
            <button
              :class="['unit-btn', { active: prefs.speedUnit === 'kmh' }]"
              @click="prefs.speedUnit = 'kmh'"
            >km/h</button>
          </label>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.playback {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--bg-app, #0e0f11);
  overflow: hidden;
}

.playback-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  background: var(--bg-surface, #1a1c23);
  border-bottom: 1px solid var(--border, #2a2c35);
  flex-shrink: 0;
}

.spacer { flex: 1; }

.back-btn, .nav-btn {
  background: var(--bg-surface, #1a1c23);
  border: 1px solid var(--border, #2a2c35);
  color: var(--text-2, #9199a6);
  border-radius: var(--radius, 4px);
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
}
.back-btn:hover, .nav-btn:hover { color: var(--text-1, #e2e4ea); background: var(--bg-hover, rgba(255,255,255,0.06)); }

.clip-date { font-size: 14px; font-weight: 500; color: var(--text-1, #e2e4ea); }
.clip-reason { font-size: 12px; color: var(--text-2, #9199a6); }

.kind-badge {
  display: inline-block;
  padding: 1px 7px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.kind-sentry { background: rgba(245,158,11,0.15); color: var(--kind-sentry, #f59e0b); }
.kind-saved  { background: rgba(96,165,250,0.15); color: var(--kind-saved, #60a5fa); }
.kind-recent { background: rgba(148,163,184,0.1); color: var(--kind-recent, #94a3b8); }

.player-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.video-area {
  flex: 1;
  min-height: 0;
  position: relative;
}

/* Floating map PiP */
.map-pip {
  position: absolute;
  bottom: 12px;
  right: 12px;
  width: 220px;
  height: 160px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid rgba(255,255,255,0.15);
  box-shadow: 0 4px 20px rgba(0,0,0,0.5);
  z-index: 10;
}

.map-close {
  position: absolute;
  top: 4px;
  right: 4px;
  background: rgba(19,20,26,0.85);
  border: none;
  color: var(--text-2, #9199a6);
  border-radius: 3px;
  width: 18px;
  height: 18px;
  font-size: 10px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 11;
}
.map-close:hover { color: var(--text-1, #e2e4ea); }

.map-show-btn {
  position: absolute;
  bottom: 12px;
  right: 12px;
  background: rgba(19,20,26,0.85);
  border: 1px solid var(--border, #2a2c35);
  color: var(--text-2, #9199a6);
  border-radius: 5px;
  padding: 5px 8px;
  font-size: 14px;
  cursor: pointer;
  z-index: 10;
}
.map-show-btn:hover { color: var(--text-1, #e2e4ea); }

.options-bar {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 6px 14px;
  background: var(--bg-surface, #1a1c23);
  border-top: 1px solid var(--border, #2a2c35);
  flex-shrink: 0;
}

.opt-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-2, #9199a6);
  cursor: pointer;
  user-select: none;
}

.unit-toggle { gap: 4px; }

.unit-btn {
  background: none;
  border: 1px solid var(--border, #2a2c35);
  color: var(--text-3, #565d6a);
  border-radius: 3px;
  padding: 1px 5px;
  font-size: 11px;
  cursor: pointer;
}
.unit-btn.active {
  background: rgba(79,142,247,0.12);
  color: var(--accent, #4f8ef7);
  border-color: var(--accent, #4f8ef7);
}

.state-msg {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-2, #9199a6);
  font-size: 14px;
}
.state-msg.error { color: #f87171; }
</style>
