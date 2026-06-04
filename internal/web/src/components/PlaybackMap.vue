<script setup lang="ts">
import { onMounted, onUnmounted, watch, ref } from 'vue'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import type { ClipTrackPoint } from '@/types'
import { initPMTilesProtocol, resolveMapStyle } from '@/lib/map'
import { computeBounds } from '@/lib/geo'

const props = defineProps<{
  track: ClipTrackPoint[]  // ordered GPS points for this clip
  currentLat: number | null
  currentLon: number | null
  currentHeading: number | null
  headStub: [number, number][] | null
  tailStub: [number, number][] | null
}>()

const emit = defineEmits<{
  // Geographic point under the pointer; the parent maps it to a track time.
  scrub: [lngLat: { lng: number; lat: number }]
  seek: [lngLat: { lng: number; lat: number }]
}>()

const container = ref<HTMLDivElement>()
let map: maplibregl.Map | null = null
const TRACK_SRC = 'pb-track'
const HIT_LAYER = 'pb-hit'
const DOT_SRC = 'pb-dot'
const STUB_SRC = 'pb-stub'
const STUB_HIT_LAYER = 'pb-stub-hit'
// Grabbable/tappable layers. The hit line width is also the proximity threshold.
const GRAB_LAYERS = ['pb-dot', HIT_LAYER, STUB_HIT_LAYER]

function trackAsRuns() {
  return [props.track.map((p) => ({ ts: p.ts, lat: p.lat, lon: p.lon, clip_id: 0 }))]
}

function emptyFC(): GeoJSON.FeatureCollection {
  return { type: 'FeatureCollection', features: [] }
}

function stubFC(): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = []
  for (const seg of [props.headStub, props.tailStub]) {
    if (seg && seg.length >= 2) {
      features.push({
        type: 'Feature',
        geometry: { type: 'LineString', coordinates: seg },
        properties: {},
      })
    }
  }
  return { type: 'FeatureCollection', features }
}

function dotFC(lat: number, lon: number): GeoJSON.FeatureCollection {
  return {
    type: 'FeatureCollection',
    features: [{ type: 'Feature', geometry: { type: 'Point', coordinates: [lon, lat] }, properties: {} }],
  }
}

function trackFC(): GeoJSON.FeatureCollection {
  if (props.track.length < 2) return emptyFC()
  return {
    type: 'FeatureCollection',
    features: [{
      type: 'Feature',
      geometry: {
        type: 'LineString',
        coordinates: props.track.map((p) => [p.lon, p.lat]),
      },
      properties: {},
    }],
  }
}

onMounted(async () => {
  if (!container.value || props.track.length === 0) return
  initPMTilesProtocol()
  const style = await resolveMapStyle()

  map = new maplibregl.Map({
    container: container.value,
    style,
    zoom: 14,
    center: [props.track[0].lon, props.track[0].lat],
    fadeDuration: 0,
    attributionControl: false,
  })

  map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right')

  map.on('load', () => {
    if (!map) return

    // Stub source: dashed/faded estimated segments at the head and tail of the
    // clip, rendered below the solid track so the real path wins at junctions.
    map.addSource(STUB_SRC, { type: 'geojson', data: stubFC() })
    map.addLayer({
      id: 'pb-stub-line',
      type: 'line',
      source: STUB_SRC,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': '#4f8ef7', 'line-width': 2, 'line-opacity': 0.4, 'line-dasharray': [3, 3] },
    })
    map.addLayer({
      id: STUB_HIT_LAYER,
      type: 'line',
      source: STUB_SRC,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': '#000', 'line-width': 32, 'line-opacity': 0 },
    })

    map.addSource(TRACK_SRC, { type: 'geojson', data: trackFC() })
    map.addLayer({
      id: 'pb-track-line',
      type: 'line',
      source: TRACK_SRC,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': '#4f8ef7', 'line-width': 2.5, 'line-opacity': 0.7 },
    })

    // Invisible wide hit area for easy tapping/grabbing of the thin path.
    // ~32px ≈ ±1em proximity threshold for registering taps/drags.
    map.addLayer({
      id: HIT_LAYER,
      type: 'line',
      source: TRACK_SRC,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': '#000', 'line-width': 32, 'line-opacity': 0 },
    })

    map.addSource(DOT_SRC, { type: 'geojson', data: emptyFC() })
    map.addLayer({
      id: 'pb-dot',
      type: 'circle',
      source: DOT_SRC,
      paint: {
        'circle-radius': 7,
        'circle-color': '#fff',
        'circle-stroke-width': 2.5,
        'circle-stroke-color': '#4f8ef7',
      },
    })

    // Fit to the track
    const bounds = computeBounds(trackAsRuns(), [])
    if (bounds) map.fitBounds(bounds, { padding: 30, maxZoom: 16, duration: 0 })

    // ── seek interactions: drag the dot/path to scrub, tap to jump ────────────
    const canvas = map.getCanvas()
    let dragging = false

    function lngLatOf(e: maplibregl.MapMouseEvent | maplibregl.MapTouchEvent) {
      return { lng: e.lngLat.lng, lat: e.lngLat.lat }
    }
    function onDragMove(e: maplibregl.MapMouseEvent | maplibregl.MapTouchEvent) {
      emit('scrub', lngLatOf(e))
    }
    function onMouseUp(e: maplibregl.MapMouseEvent) {
      dragging = false
      canvas.style.cursor = ''
      map?.off('mousemove', onDragMove)
      emit('seek', lngLatOf(e))
    }
    function onTouchEnd(e: maplibregl.MapTouchEvent) {
      dragging = false
      map?.off('touchmove', onDragMove)
      emit('seek', lngLatOf(e))
    }
    function startMouseDrag(e: maplibregl.MapMouseEvent) {
      e.preventDefault() // suppress map pan while scrubbing
      dragging = true
      canvas.style.cursor = 'grabbing'
      map?.on('mousemove', onDragMove)
      map?.once('mouseup', onMouseUp)
    }
    function startTouchDrag(e: maplibregl.MapTouchEvent) {
      if (e.points.length !== 1) return // let pinch-zoom through
      e.preventDefault()
      dragging = true
      map?.on('touchmove', onDragMove)
      map?.once('touchend', onTouchEnd)
    }

    for (const layer of GRAB_LAYERS) {
      map.on('mousedown', layer, startMouseDrag)
      map.on('touchstart', layer, startTouchDrag)
      map.on('mouseenter', layer, () => {
        if (!dragging) canvas.style.cursor = 'grab'
      })
      map.on('mouseleave', layer, () => {
        if (!dragging) canvas.style.cursor = ''
      })
    }
  })
})

onUnmounted(() => {
  map?.remove()
  map = null
})

// Update the dot when currentLat/Lon changes.
watch(
  () => [props.currentLat, props.currentLon] as const,
  ([lat, lon]) => {
    if (!map?.loaded() || lat === null || lon === null) return
    ;(map.getSource(DOT_SRC) as maplibregl.GeoJSONSource)?.setData(dotFC(lat, lon))
  },
)

// Update stub geometry when stubs arrive (after video duration loads).
watch(
  () => [props.headStub, props.tailStub] as const,
  () => {
    if (!map?.loaded()) return
    ;(map.getSource(STUB_SRC) as maplibregl.GeoJSONSource)?.setData(stubFC())
  },
)
</script>

<template>
  <div ref="container" class="pb-map" />
</template>

<style scoped>
.pb-map {
  width: 100%;
  height: 100%;
}
</style>
