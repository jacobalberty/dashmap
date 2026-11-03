<script setup lang="ts">
import { onMounted, onUnmounted, watch, ref } from 'vue'
import * as maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { useRouter } from 'vue-router'
import { useBrowseStore } from '@/stores/browse'
import { initPMTilesProtocol, resolveMapStyle, addArrowImage, addPinImage } from '@/lib/map'
import { runsToGeoJSON, runEndpointsGeoJSON, markersToGeoJSON, spiderGeoJSON, computeBounds } from '@/lib/geo'
import type { ClipMarker } from '@/types'

const store = useBrowseStore()
const router = useRouter()
const container = ref<HTMLDivElement>()
const showTrack = ref(true)
const noticeText = ref<string | null>(null)

let map: maplibregl.Map | null = null
let spiderOpen = false
let popup: maplibregl.Popup | null = null

const TRACK_SRC = 'track'
const ENDPOINTS_SRC = 'endpoints'
const MARKERS_SRC = 'markers'
const SPIDER_SRC = 'spider'
const SPIDER_LEGS_SRC = 'spider-legs'

onMounted(async () => {
  initPMTilesProtocol()
  const style = await resolveMapStyle()

  map = new maplibregl.Map({
    container: container.value!,
    style,
    zoom: 4,
    center: [-98, 39],
    fadeDuration: 150,
  })

  map.addControl(new maplibregl.NavigationControl(), 'top-right')
  map.addControl(new maplibregl.ScaleControl({ unit: 'metric' }), 'bottom-right')

  map.on('load', () => {
    if (!map) return
    addArrowImage(map)
    addPinImage(map, 'pin-start', '#34d399')
    addPinImage(map, 'pin-end', '#f87171')

    // ── Track ────────────────────────────────────────────────────────────────
    map.addSource(TRACK_SRC, { type: 'geojson', data: emptyFC() })
    map.addSource(ENDPOINTS_SRC, { type: 'geojson', data: emptyFC() })

    map.addLayer({
      id: 'track-line',
      type: 'line',
      source: TRACK_SRC,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': '#4f8ef7', 'line-width': 2.5, 'line-opacity': 0.85 },
    })

    map.addLayer({
      id: 'track-arrows',
      type: 'symbol',
      source: TRACK_SRC,
      layout: {
        'symbol-placement': 'line',
        'symbol-spacing': 60,
        'icon-image': 'track-arrow',
        'icon-size': 0.9,
        'icon-rotation-alignment': 'map',
        'icon-keep-upright': false,
        'icon-allow-overlap': false,
      },
    })

    map.addLayer({
      id: 'track-endpoints',
      type: 'symbol',
      source: ENDPOINTS_SRC,
      layout: {
        'icon-image': ['match', ['get', 'role'], 'start', 'pin-start', 'pin-end'],
        'icon-size': 1,
        'icon-allow-overlap': true,
      },
    })

    // ── Markers ──────────────────────────────────────────────────────────────
    map.addSource(MARKERS_SRC, {
      type: 'geojson',
      data: emptyFC(),
      cluster: true,
      clusterMaxZoom: 16,
      clusterRadius: 40,
    })

    // Cluster circles
    map.addLayer({
      id: 'markers-clusters',
      type: 'circle',
      source: MARKERS_SRC,
      filter: ['has', 'point_count'],
      paint: {
        'circle-color': ['step', ['get', 'point_count'], '#4f8ef7', 5, '#a78bfa', 20, '#f59e0b'],
        'circle-radius': ['step', ['get', 'point_count'], 14, 5, 18, 20, 22],
        'circle-stroke-width': 2,
        'circle-stroke-color': 'rgba(255,255,255,0.2)',
      },
    })

    map.addLayer({
      id: 'markers-cluster-count',
      type: 'symbol',
      source: MARKERS_SRC,
      filter: ['has', 'point_count'],
      layout: {
        'text-field': '{point_count_abbreviated}',
        'text-font': ['Noto Sans Bold'],
        'text-size': 11,
      },
      paint: { 'text-color': '#fff' },
    })

    // Individual (unclustered) markers
    map.addLayer({
      id: 'markers-unclustered',
      type: 'circle',
      source: MARKERS_SRC,
      filter: ['!', ['has', 'point_count']],
      paint: {
        'circle-radius': 6,
        'circle-color': [
          'match', ['get', 'kind'],
          'sentry', '#f59e0b',
          'saved', '#60a5fa',
          '#94a3b8',
        ],
        'circle-stroke-width': 2,
        'circle-stroke-color': [
          'case',
          ['==', ['get', 'id'], store.selectedClipId ?? -1],
          '#fff',
          'rgba(255,255,255,0.25)',
        ],
        'circle-stroke-opacity': 1,
      },
    })

    // ── Spider ───────────────────────────────────────────────────────────────
    map.addSource(SPIDER_SRC, { type: 'geojson', data: emptyFC() })
    map.addSource(SPIDER_LEGS_SRC, { type: 'geojson', data: emptyFC() })

    map.addLayer({
      id: 'spider-legs',
      type: 'line',
      source: SPIDER_LEGS_SRC,
      paint: { 'line-color': 'rgba(255,255,255,0.25)', 'line-width': 1 },
    })

    map.addLayer({
      id: 'spider-markers',
      type: 'circle',
      source: SPIDER_SRC,
      paint: {
        'circle-radius': 7,
        'circle-color': [
          'match', ['get', 'kind'],
          'sentry', '#f59e0b',
          'saved', '#60a5fa',
          '#94a3b8',
        ],
        'circle-stroke-width': 2,
        'circle-stroke-color': 'rgba(255,255,255,0.5)',
      },
    })

    // ── Click handlers ───────────────────────────────────────────────────────
    map.on('click', 'markers-clusters', onClusterClick)
    map.on('click', 'markers-unclustered', onMarkerClick)
    map.on('click', 'spider-markers', onSpiderMarkerClick)
    map.on('dblclick', 'markers-unclustered', onMarkerDblClick)
    map.on('dblclick', 'spider-markers', onSpiderMarkerDblClick)
    map.on('click', onMapClick)

    // Cursor changes
    for (const layer of ['markers-clusters', 'markers-unclustered', 'spider-markers']) {
      map.on('mouseenter', layer, () => { if (map) map.getCanvas().style.cursor = 'pointer' })
      map.on('mouseleave', layer, () => { if (map) map.getCanvas().style.cursor = '' })
    }
  })
})

onUnmounted(() => {
  popup?.remove()
  map?.remove()
})

// ── Watchers ────────────────────────────────────────────────────────────────

watch([() => store.trackRuns, () => store.markers], ([runs, markers]) => {
  if (!map?.loaded()) return
  updateMapData(runs, markers)
})

watch(() => store.selectedClipId, (id) => {
  updateSelectedPaint(id)
})

watch(showTrack, (v) => {
  if (!map?.loaded()) return
  const vis = v ? 'visible' : 'none'
  for (const l of ['track-line', 'track-arrows', 'track-endpoints']) {
    if (map.getLayer(l)) map.setLayoutProperty(l, 'visibility', vis)
  }
})

watch(() => store.markersCapped, (capped) => {
  noticeText.value = capped ? 'Showing a sample of markers — narrow the range for full detail.' : null
})

watch(() => store.trackMeta, (meta) => {
  if (meta?.simplified && !store.markersCapped) {
    noticeText.value = 'Track simplified — narrow the range for full resolution.'
  }
})

// ── Helpers ──────────────────────────────────────────────────────────────────

function emptyFC() {
  return { type: 'FeatureCollection' as const, features: [] }
}

function updateMapData(runs: typeof store.trackRuns, markers: typeof store.markers) {
  if (!map) return
  const trackFC = runsToGeoJSON(runs)
  const endpointsFC = runEndpointsGeoJSON(runs)
  const markersFC = markersToGeoJSON(markers)

  ;(map.getSource(TRACK_SRC) as maplibregl.GeoJSONSource)?.setData(trackFC)
  ;(map.getSource(ENDPOINTS_SRC) as maplibregl.GeoJSONSource)?.setData(endpointsFC)
  ;(map.getSource(MARKERS_SRC) as maplibregl.GeoJSONSource)?.setData(markersFC)

  clearSpider()

  const bounds = computeBounds(runs, markers)
  if (bounds) {
    map.fitBounds(bounds, { padding: 60, maxZoom: 15, duration: 500 })
  }
}

function updateSelectedPaint(id: number | null) {
  if (!map?.getLayer('markers-unclustered')) return
  map.setPaintProperty('markers-unclustered', 'circle-stroke-color', [
    'case',
    ['==', ['get', 'id'], id ?? -1],
    '#fff',
    'rgba(255,255,255,0.25)',
  ])
  map.setPaintProperty('markers-unclustered', 'circle-stroke-width', [
    'case',
    ['==', ['get', 'id'], id ?? -1],
    3,
    2,
  ])
}

async function onClusterClick(e: maplibregl.MapMouseEvent) {
  if (!map) return
  e.preventDefault()
  const features = map.queryRenderedFeatures(e.point, { layers: ['markers-clusters'] })
  if (!features.length) return
  const clusterId = features[0].properties?.cluster_id as number | undefined
  if (!clusterId) return
  const src = map.getSource(MARKERS_SRC) as maplibregl.GeoJSONSource

  // Check if we can zoom further.
  const zoom = await new Promise<number>((res) => src.getClusterExpansionZoom(clusterId, (_, z) => res(z ?? 20)))
  if (zoom <= map.getMaxZoom()) {
    map.easeTo({ center: (features[0].geometry as GeoJSON.Point).coordinates as [number, number], zoom })
    return
  }

  // At max zoom — spiderfy co-located leaves.
  const leaves = await new Promise<ClipMarker[]>((res) =>
    src.getClusterLeaves(clusterId, 40, 0, (_, f) => {
      res((f ?? []).map((feat) => feat.properties as unknown as ClipMarker))
    }),
  )
  const center = (features[0].geometry as GeoJSON.Point).coordinates as [number, number]
  openSpider(center, leaves)
}

function onMarkerClick(e: maplibregl.MapMouseEvent) {
  e.preventDefault()
  const f = map?.queryRenderedFeatures(e.point, { layers: ['markers-unclustered'] })[0]
  if (!f) return
  const id = f.properties?.id as number
  store.selectClip(id)
  showPopup(id, (f.geometry as GeoJSON.Point).coordinates as [number, number])
}

function onMarkerDblClick(e: maplibregl.MapMouseEvent) {
  e.preventDefault()
  const f = map?.queryRenderedFeatures(e.point, { layers: ['markers-unclustered'] })[0]
  if (f) {
    store.selectClip(f.properties?.id as number)
    router.push(`/clip/${f.properties?.id}`)
  }
}

function onSpiderMarkerClick(e: maplibregl.MapMouseEvent) {
  e.preventDefault()
  const f = map?.queryRenderedFeatures(e.point, { layers: ['spider-markers'] })[0]
  if (!f) return
  const id = f.properties?.id as number
  store.selectClip(id)
  showPopup(id, (f.geometry as GeoJSON.Point).coordinates as [number, number])
}

function onSpiderMarkerDblClick(e: maplibregl.MapMouseEvent) {
  e.preventDefault()
  const f = map?.queryRenderedFeatures(e.point, { layers: ['spider-markers'] })[0]
  if (f) {
    store.selectClip(f.properties?.id as number)
    router.push(`/clip/${f.properties?.id}`)
  }
}

function onMapClick(e: maplibregl.MapMouseEvent) {
  // Close spider if clicked outside any marker/spider.
  const hit = map?.queryRenderedFeatures(e.point, {
    layers: ['markers-clusters', 'markers-unclustered', 'spider-markers'],
  })
  if (!hit?.length) {
    clearSpider()
    popup?.remove()
  }
}

function openSpider(center: [number, number], leaves: ClipMarker[]) {
  if (!map) return
  clearSpider()
  spiderOpen = true

  const zoom = map.getZoom()
  const spiderFC = spiderGeoJSON(center, leaves, zoom)

  // Build legs (lines from center to each spider point).
  const legsFC: GeoJSON.FeatureCollection<GeoJSON.LineString> = {
    type: 'FeatureCollection',
    features: spiderFC.features.map((f) => ({
      type: 'Feature',
      geometry: {
        type: 'LineString',
        coordinates: [center, (f.geometry as GeoJSON.Point).coordinates],
      },
      properties: {},
    })),
  }
  ;(map.getSource(SPIDER_SRC) as maplibregl.GeoJSONSource)?.setData(spiderFC)
  ;(map.getSource(SPIDER_LEGS_SRC) as maplibregl.GeoJSONSource)?.setData(legsFC)
}

function clearSpider() {
  if (!spiderOpen) return
  spiderOpen = false
  ;(map?.getSource(SPIDER_SRC) as maplibregl.GeoJSONSource)?.setData(emptyFC())
  ;(map?.getSource(SPIDER_LEGS_SRC) as maplibregl.GeoJSONSource)?.setData(emptyFC())
}

function showPopup(clipId: number, coords: [number, number]) {
  popup?.remove()
  const clip = store.clips.find((c) => c.id === clipId)
  if (!clip) return

  const kindColor = clip.kind === 'sentry' ? '#f59e0b' : clip.kind === 'saved' ? '#60a5fa' : '#94a3b8'
  const time = new Date(clip.clip_ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  const city = clip.city ? `<div style="font-size:10px;color:#9199a6;margin-top:2px">${clip.city}</div>` : ''
  const reason = clip.reason ? `<div style="font-size:10px;color:#9199a6">${clip.reason}</div>` : ''

  const html = `
    <div style="background:#1a1c23;border-radius:6px;padding:8px 10px;min-width:140px;font-family:system-ui">
      <div style="display:flex;align-items:center;gap:6px;margin-bottom:4px">
        <span style="font-size:12px;font-weight:600;color:#e2e4ea">${time}</span>
        <span style="font-size:10px;font-weight:700;color:${kindColor};text-transform:uppercase;letter-spacing:.5px">${clip.kind}</span>
      </div>
      ${reason}${city}
      <button onclick="window.__dashmapPlay(${clipId})"
        style="margin-top:6px;width:100%;background:#4f8ef7;border:none;border-radius:4px;padding:4px 0;font-size:11px;color:#fff;cursor:pointer">
        ▶ Play
      </button>
    </div>`

  // Bind the play handler onto window so the inline onclick can find it.
  ;(window as unknown as Record<string, unknown>).__dashmapPlay = (id: number) => {
    store.selectClip(id)
    router.push(`/clip/${id}`)
    popup?.remove()
  }

  popup = new maplibregl.Popup({
    closeButton: false,
    maxWidth: 'none',
    className: 'dashmap-popup',
  })
    .setLngLat(coords)
    .setHTML(html)
    .addTo(map!)
}
</script>

<template>
  <div class="map-container">
    <div ref="container" class="map" />

    <div class="map-controls">
      <button
        :class="['track-toggle', { active: showTrack }]"
        @click="showTrack = !showTrack"
        title="Toggle track"
      >
        {{ showTrack ? '━ track' : '━ track (off)' }}
      </button>
    </div>

    <div v-if="noticeText" class="map-notice">
      {{ noticeText }}
      <button class="notice-close" @click="noticeText = null">✕</button>
    </div>

    <div v-if="store.mapLoading" class="map-loading-overlay">
      <span>Loading…</span>
    </div>

    <div v-if="store.hasRange && store.markers.length === 0 && store.trackRuns.length === 0 && !store.mapLoading"
         class="map-empty">
      No GPS data in this range.
    </div>
  </div>
</template>

<style scoped>
.map-container {
  position: relative;
  width: 100%;
  height: 100%;
}

.map {
  width: 100%;
  height: 100%;
}

.map-controls {
  position: absolute;
  top: 10px;
  left: 10px;
  z-index: 5;
}

.track-toggle {
  background: rgba(19,20,26,0.88);
  border: 1px solid var(--border);
  color: var(--text-2);
  border-radius: var(--radius);
  padding: 4px 10px;
  font-size: 11px;
  cursor: pointer;
  backdrop-filter: blur(4px);
}
.track-toggle:hover { color: var(--text-1); }
.track-toggle.active { color: #4f8ef7; }

.map-notice {
  position: absolute;
  bottom: 44px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(19,20,26,0.9);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 6px 12px 6px 10px;
  font-size: 11px;
  color: var(--text-2);
  display: flex;
  align-items: center;
  gap: 8px;
  backdrop-filter: blur(4px);
  z-index: 5;
}

.notice-close {
  background: none;
  border: none;
  color: var(--text-3);
  font-size: 11px;
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.map-loading-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(14,15,17,0.35);
  font-size: 13px;
  color: var(--text-2);
  pointer-events: none;
  z-index: 4;
}

.map-empty {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  background: rgba(19,20,26,0.85);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 12px 20px;
  font-size: 12px;
  color: var(--text-2);
  pointer-events: none;
}
</style>

<style>
/* Global MapLibre popup override */
.maplibregl-popup-content {
  background: transparent !important;
  padding: 0 !important;
  box-shadow: 0 4px 20px rgba(0,0,0,0.5) !important;
  border-radius: 6px !important;
}
.maplibregl-popup-tip { display: none !important; }
</style>
