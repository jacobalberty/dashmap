<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getClipDetail } from '@/lib/api'
import { formatLocalDate, localTz } from '@/lib/datetime'
import type { ClipDetail } from '@/types'

const route = useRoute()
const router = useRouter()
const clip = ref<ClipDetail | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const tz = localTz()

onMounted(async () => {
  const id = Number(route.params.id)
  if (!id) { error.value = 'Invalid clip ID'; loading.value = false; return }
  try {
    clip.value = await getClipDetail(id)
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="playback">
    <div class="playback-header">
      <button class="back-btn" @click="router.push('/')">← back to map</button>
      <template v-if="clip">
        <span class="clip-date">{{ formatLocalDate(clip.clip_ts, tz) }}</span>
        <span :class="['kind-badge', `kind-${clip.kind}`]">{{ clip.kind }}</span>
        <span v-if="clip.event?.reason" class="clip-reason">{{ clip.event.reason }}</span>
      </template>
    </div>

    <div v-if="loading" class="state-msg">Loading…</div>
    <div v-else-if="error" class="state-msg error">{{ error }}</div>
    <template v-else-if="clip">
      <div class="segments-grid">
        <div v-for="seg in clip.segments" :key="seg.segment_id" class="segment-card">
          <div class="seg-label">{{ seg.camera }}</div>
          <video
            :src="seg.media_url"
            controls
            preload="metadata"
            class="seg-video"
          />
          <a :href="seg.download_url" class="seg-dl" download>⬇ download</a>
        </div>
      </div>

      <div class="clip-actions">
        <a :href="clip.download_url" class="dl-btn" download>⬇ Download all cameras (zip)</a>
      </div>

      <p class="story-note">Full multi-camera playback with SEI overlay is coming in Story 06.</p>
    </template>
  </div>
</template>

<style scoped>
.playback {
  height: 100%;
  overflow-y: auto;
  background: var(--bg-app);
  padding: 20px 24px;
}

.playback-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.back-btn {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  color: var(--text-2);
  border-radius: var(--radius);
  padding: 5px 10px;
  font-size: 12px;
  cursor: pointer;
}
.back-btn:hover { color: var(--text-1); background: var(--bg-hover); }

.clip-date { font-size: 14px; font-weight: 500; }
.clip-reason { font-size: 12px; color: var(--text-2); }

.kind-badge {
  display: inline-block;
  padding: 1px 7px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.kind-sentry { background: rgba(245,158,11,0.15); color: var(--kind-sentry); }
.kind-saved { background: rgba(96,165,250,0.15); color: var(--kind-saved); }
.kind-recent { background: rgba(148,163,184,0.1); color: var(--kind-recent); }

.segments-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.segment-card {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow: hidden;
}

.seg-label {
  font-size: 11px;
  color: var(--text-2);
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.seg-video {
  width: 100%;
  display: block;
  background: #000;
}

.seg-dl {
  display: block;
  padding: 6px 10px;
  font-size: 11px;
  color: var(--text-3);
  text-decoration: none;
  border-top: 1px solid var(--border);
}
.seg-dl:hover { color: var(--accent); }

.clip-actions { margin-bottom: 16px; }

.dl-btn {
  display: inline-block;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  color: var(--text-2);
  border-radius: var(--radius);
  padding: 6px 14px;
  font-size: 12px;
  text-decoration: none;
  cursor: pointer;
}
.dl-btn:hover { color: var(--accent); border-color: var(--accent); }

.state-msg { color: var(--text-2); padding: 40px 0; text-align: center; }
.state-msg.error { color: #f87171; }

.story-note {
  font-size: 11px;
  color: var(--text-3);
  font-style: italic;
}
</style>
