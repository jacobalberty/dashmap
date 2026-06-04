import { ref, onUnmounted } from 'vue'

// requestVideoFrameCallback is in the DOM lib since TypeScript 4.9 but the
// @vue/tsconfig dom preset may not expose it; augment here to be safe.
interface VideoFrameMetadata {
  mediaTime: number
  presentationTime: DOMHighResTimeStamp
  expectedDisplayTime: DOMHighResTimeStamp
  width: number
  height: number
  presentedFrames: number
  processingDuration?: number
}

interface RVFCVideo extends HTMLVideoElement {
  requestVideoFrameCallback(
    cb: (now: DOMHighResTimeStamp, meta: VideoFrameMetadata) => void,
  ): number
  cancelVideoFrameCallback(handle: number): void
}

function hasRVFC(v: HTMLVideoElement): v is RVFCVideo {
  return 'requestVideoFrameCallback' in v
}

// ── composable ────────────────────────────────────────────────────────────────

export function usePlaybackSync() {
  // Index-matched to clip.segments order; index 0 = front = master.
  const videos: (HTMLVideoElement | null)[] = []

  const playing = ref(false)
  const currentTime = ref(0)
  const duration = ref(0)
  const rate = ref(1)

  let rvfcHandle = -1
  let rafHandle = -1

  // Scrub session: while the user drags the seek bar / map dot, the sync loop is
  // frozen and the videos are paused so live previewing is smooth; on commit
  // (seek) playback resumes if it was running.
  const scrubbing = ref(false)
  let scrubWasPlaying = false
  let scrubRaf = -1
  let scrubTarget = 0

  function master(): HTMLVideoElement | null {
    return videos[0] ?? null
  }

  // ── sync loop ───────────────────────────────────────────────────────────────

  function syncSlaves(masterT: number) {
    for (let i = 1; i < videos.length; i++) {
      const v = videos[i]
      if (!v) continue
      if (Math.abs(v.currentTime - masterT) > 0.1) {
        v.currentTime = masterT
      }
    }
  }

  function onFrame(_now: DOMHighResTimeStamp, meta: VideoFrameMetadata) {
    const t = meta.mediaTime
    currentTime.value = t
    syncSlaves(t)
    if (playing.value) {
      const m = master()
      if (m && hasRVFC(m)) {
        rvfcHandle = m.requestVideoFrameCallback(onFrame)
      }
    }
  }

  function onRaf() {
    const m = master()
    if (!m) return
    const t = m.currentTime
    currentTime.value = t
    syncSlaves(t)
    if (playing.value) {
      rafHandle = requestAnimationFrame(onRaf)
    }
  }

  function startLoop() {
    stopLoop()
    const m = master()
    if (!m) return
    if (hasRVFC(m)) {
      rvfcHandle = m.requestVideoFrameCallback(onFrame)
    } else {
      rafHandle = requestAnimationFrame(onRaf)
    }
  }

  function stopLoop() {
    if (rvfcHandle >= 0) {
      const m = master()
      if (m && hasRVFC(m)) m.cancelVideoFrameCallback(rvfcHandle)
      rvfcHandle = -1
    }
    if (rafHandle >= 0) {
      cancelAnimationFrame(rafHandle)
      rafHandle = -1
    }
  }

  // ── controls ────────────────────────────────────────────────────────────────

  function play() {
    const m = master()
    if (!m) return
    playing.value = true
    for (const v of videos) v?.play().catch(() => {})
    startLoop()
  }

  function pause() {
    playing.value = false
    stopLoop()
    for (const v of videos) v?.pause()
  }

  function toggle() {
    if (playing.value) pause()
    else play()
  }

  function clampTime(t: number): number {
    return Math.max(0, Math.min(t, duration.value || 0))
  }

  function flushScrub() {
    scrubRaf = -1
    for (const v of videos) {
      if (v) v.currentTime = scrubTarget
    }
  }

  // Continuous live preview while dragging. Begins a scrub session on the first
  // call: freezes the sync loop and pauses media (remembering play intent) so the
  // dot/HUD/videos all track the drag without the loop fighting currentTime.
  function scrub(t: number) {
    const clamped = clampTime(t)
    if (!scrubbing.value) {
      scrubbing.value = true
      scrubWasPlaying = playing.value
      stopLoop()
      for (const v of videos) v?.pause()
    }
    currentTime.value = clamped
    scrubTarget = clamped
    if (scrubRaf < 0) scrubRaf = requestAnimationFrame(flushScrub)
  }

  function endScrub() {
    if (scrubRaf >= 0) {
      cancelAnimationFrame(scrubRaf)
      scrubRaf = -1
    }
    scrubbing.value = false
    const t = currentTime.value
    for (const v of videos) {
      if (v) v.currentTime = t
    }
    if (scrubWasPlaying) {
      for (const v of videos) v?.play().catch(() => {})
      startLoop()
    }
    scrubWasPlaying = false
  }

  function seek(t: number) {
    const clamped = clampTime(t)
    currentTime.value = clamped
    if (scrubbing.value) {
      // Commit an in-progress scrub session (resumes playback if it was playing).
      scrubTarget = clamped
      endScrub()
      return
    }
    for (const v of videos) {
      if (v) v.currentTime = clamped
    }
  }

  function seekBy(delta: number) {
    seek(currentTime.value + delta)
  }

  function setRate(r: number) {
    rate.value = r
    for (const v of videos) {
      if (v) v.playbackRate = r
    }
  }

  // ── registration ────────────────────────────────────────────────────────────

  function registerVideo(index: number, el: HTMLVideoElement | null) {
    videos[index] = el
    if (el && index === 0) {
      el.addEventListener('loadedmetadata', () => {
        duration.value = el.duration || 0
      })
      el.addEventListener('ended', () => {
        playing.value = false
        stopLoop()
      })
      el.addEventListener('seeked', () => {
        // During a scrub session we drive currentTime + slaves directly; ignore
        // the per-frame seek events so they don't fight the scrub target.
        if (scrubbing.value) return
        currentTime.value = el.currentTime
        syncSlaves(el.currentTime)
      })
      // Restart the loop on the new master element if playback was active.
      if (playing.value) startLoop()
    }
    if (el && rate.value !== 1) {
      el.playbackRate = rate.value
    }
  }

  function getVideo(index: number): HTMLVideoElement | null {
    return videos[index] ?? null
  }

  function reset() {
    pause()
    if (scrubRaf >= 0) {
      cancelAnimationFrame(scrubRaf)
      scrubRaf = -1
    }
    scrubbing.value = false
    scrubWasPlaying = false
    videos.length = 0
    currentTime.value = 0
    duration.value = 0
  }

  onUnmounted(() => {
    stopLoop()
  })

  return {
    registerVideo,
    getVideo,
    reset,
    playing,
    currentTime,
    duration,
    rate,
    play,
    pause,
    toggle,
    seek,
    scrub,
    seekBy,
    setRate,
  }
}
