<script setup lang="ts">
import { ref, watch } from 'vue'
import { formatDuration } from '@/lib/datetime'

const props = defineProps<{
  playing: boolean
  currentTime: number
  duration: number
  rate: number
  downloadUrl: string
}>()

const emit = defineEmits<{
  toggle: []
  scrub: [t: number]
  seek: [t: number]
  seekBy: [delta: number]
  setRate: [r: number]
  fullscreen: []
}>()

// Local seek value for smooth seek-bar display during drag.
const seekValue = ref(props.currentTime)
const isDragging = ref(false)

watch(
  () => props.currentTime,
  (t) => {
    if (!isDragging.value) seekValue.value = t
  },
)

function onSeekInput(e: Event) {
  isDragging.value = true
  const v = Number((e.target as HTMLInputElement).value)
  seekValue.value = v
  emit('scrub', v)
}

function onSeekChange(e: Event) {
  isDragging.value = false
  emit('seek', Number((e.target as HTMLInputElement).value))
}

const RATES = [0.5, 1, 1.5, 2, 4]
</script>

<template>
  <div class="transport">
    <button class="ctrl-btn" @click="$emit('toggle')" :aria-label="playing ? 'Pause' : 'Play'">
      {{ playing ? '⏸' : '▶' }}
    </button>
    <button class="ctrl-btn" @click="$emit('seekBy', -10)" title="-10s">⏪</button>
    <button class="ctrl-btn" @click="$emit('seekBy', 10)" title="+10s">⏩</button>

    <input
      type="range"
      class="seek-bar"
      :min="0"
      :max="duration || 0"
      :value="seekValue"
      step="0.05"
      @input="onSeekInput"
      @change="onSeekChange"
    />

    <span class="time-display">
      {{ formatDuration(currentTime) }} / {{ formatDuration(duration || 0) }}
    </span>

    <select
      class="rate-select"
      :value="rate"
      @change="$emit('setRate', Number(($event.target as HTMLSelectElement).value))"
    >
      <option v-for="r in RATES" :key="r" :value="r">{{ r }}×</option>
    </select>

    <button class="ctrl-btn" @click="$emit('fullscreen')" title="Fullscreen all">⛶</button>

    <a :href="downloadUrl" download class="ctrl-btn dl-link" title="Download all cameras (zip)">⬇</a>
  </div>
</template>

<style scoped>
.transport {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  background: var(--bg-surface, #1a1c23);
  border-top: 1px solid var(--border, #2a2c35);
  flex-shrink: 0;
}

.ctrl-btn {
  background: none;
  border: none;
  color: var(--text-2, #9199a6);
  font-size: 14px;
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 3px;
  line-height: 1;
  flex-shrink: 0;
}
.ctrl-btn:hover { color: var(--text-1, #e2e4ea); background: var(--bg-hover, rgba(255,255,255,0.06)); }

.dl-link {
  text-decoration: none;
  display: inline-block;
}

.seek-bar {
  flex: 1;
  min-width: 60px;
  accent-color: var(--accent, #4f8ef7);
  height: 3px;
  cursor: pointer;
}

.time-display {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  color: var(--text-2, #9199a6);
  white-space: nowrap;
  flex-shrink: 0;
}

.rate-select {
  background: var(--bg-surface, #1a1c23);
  border: 1px solid var(--border, #2a2c35);
  color: var(--text-2, #9199a6);
  font-size: 11px;
  border-radius: 3px;
  padding: 2px 4px;
  cursor: pointer;
  flex-shrink: 0;
}
.rate-select:hover { color: var(--text-1, #e2e4ea); }
</style>
