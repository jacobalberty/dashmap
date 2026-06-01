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
}>()

const container = ref<HTMLDivElement>()
let map: maplibregl.Map | null = null
const TRACK_SRC = 'pb-track'
const DOT_SRC = 'pb-dot'

function trackAsRuns() {
  return [props.track.map((p) => ({ ts: p.ts, lat: p.lat, lon: p.lon, clip_id: 0 }))]
}

function emptyFC(): GeoJSON.FeatureCollection {
  return { type: 'FeatureCollection', features: [] }
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

    map.addSource(TRACK_SRC, { type: 'geojson', data: trackFC() })
    map.addLayer({
      id: 'pb-track-line',
      type: 'line',
      source: TRACK_SRC,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': '#4f8ef7', 'line-width': 2.5, 'line-opacity': 0.7 },
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
