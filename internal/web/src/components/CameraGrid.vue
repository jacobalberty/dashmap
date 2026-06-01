<script setup lang="ts">
import { ref } from 'vue'
import type { ClipSegment } from '@/types'

const props = defineProps<{
  segments: ClipSegment[]
  registerVideo: (index: number, el: HTMLVideoElement | null) => void
  getVideo: (index: number) => HTMLVideoElement | null
}>()

const focusedIndex = ref<number | null>(null)
const containerRef = ref<HTMLDivElement>()

function gridCols(n: number): string {
  if (n <= 1) return 'grid-1'
  if (n === 2) return 'grid-2'
  if (n <= 4) return 'grid-4'
  return 'grid-6'
}

function onTileClick(i: number) {
  focusedIndex.value = focusedIndex.value === i ? null : i
}

function onTileDblClick(i: number) {
  const video = props.getVideo(i)
  if (video?.requestFullscreen) video.requestFullscreen().catch(() => {})
}

function fullscreenAll() {
  containerRef.value?.requestFullscreen?.().catch(() => {})
}

defineExpose({ fullscreenAll, focusIndex: (i: number) => { focusedIndex.value = i } })
</script>

<template>
  <div ref="containerRef" class="camera-grid-wrap" :class="{ 'has-focus': focusedIndex !== null }">
    <!-- main area: focused tile or all tiles in grid -->
    <div class="main-area" :class="gridCols(focusedIndex === null ? segments.length : 1)">
      <template v-if="focusedIndex === null">
        <div
          v-for="(seg, i) in segments"
          :key="seg.segment_id"
          class="tile"
          @click="onTileClick(i)"
          @dblclick.prevent="onTileDblClick(i)"
        >
          <span class="cam-label">{{ seg.camera.replace(/_/g, ' ') }}</span>
          <video
            :ref="(el) => registerVideo(i, el as HTMLVideoElement | null)"
            :src="seg.media_url"
            preload="metadata"
            muted
            playsinline
            class="cam-video"
          />
        </div>
      </template>
      <template v-else>
        <div class="tile focused-tile" @dblclick.prevent="onTileDblClick(focusedIndex!)">
          <span class="cam-label">{{ segments[focusedIndex!].camera.replace(/_/g, ' ') }}</span>
          <video
            :ref="(el) => registerVideo(focusedIndex!, el as HTMLVideoElement | null)"
            :src="segments[focusedIndex!].media_url"
            preload="metadata"
            muted
            playsinline
            class="cam-video"
          />
        </div>
      </template>
    </div>

    <!-- thumbnail strip when a tile is focused -->
    <div v-if="focusedIndex !== null" class="thumb-strip">
      <div
        v-for="(seg, i) in segments"
        :key="seg.segment_id"
        class="thumb-tile"
        :class="{ active: i === focusedIndex }"
        @click="onTileClick(i)"
      >
        <span class="cam-label-small">{{ seg.camera.replace(/_/g, ' ') }}</span>
        <video
          v-if="i !== focusedIndex"
          :ref="(el) => registerVideo(i, el as HTMLVideoElement | null)"
          :src="seg.media_url"
          preload="metadata"
          muted
          playsinline
          class="thumb-video"
        />
        <div v-else class="thumb-active-indicator" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.camera-grid-wrap {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #000;
}

.main-area {
  flex: 1;
  min-height: 0;
  display: grid;
  gap: 2px;
}
.grid-1 { grid-template-columns: 1fr; }
.grid-2 { grid-template-columns: 1fr 1fr; }
.grid-4 { grid-template-columns: 1fr 1fr; }
.grid-6 { grid-template-columns: 1fr 1fr 1fr; }

.tile {
  position: relative;
  background: #0a0a0a;
  cursor: pointer;
  overflow: hidden;
}
.tile:hover .cam-label { opacity: 1; }

.focused-tile {
  cursor: default;
}

.cam-label {
  position: absolute;
  top: 6px;
  left: 8px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: rgba(255,255,255,0.55);
  opacity: 0;
  transition: opacity 0.15s;
  z-index: 2;
  pointer-events: none;
}
.focused-tile .cam-label { opacity: 1; }

.cam-video {
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
}

.thumb-strip {
  flex: 0 0 72px;
  display: flex;
  gap: 2px;
  background: #000;
}

.thumb-tile {
  flex: 1;
  position: relative;
  cursor: pointer;
  background: #111;
  overflow: hidden;
}
.thumb-tile.active {
  outline: 2px solid var(--accent, #4f8ef7);
  outline-offset: -2px;
}

.cam-label-small {
  position: absolute;
  bottom: 4px;
  left: 4px;
  font-size: 9px;
  color: rgba(255,255,255,0.5);
  z-index: 2;
  pointer-events: none;
  text-transform: uppercase;
}

.thumb-video {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.thumb-active-indicator {
  width: 100%;
  height: 100%;
  background: rgba(79,142,247,0.15);
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
