import type { Feature, FeatureCollection, LineString, Point } from 'geojson'
import type { TrackPoint, ClipMarker } from '@/types'

export type KindColor = 'sentry' | 'saved' | 'recent'

// Build a GeoJSON FeatureCollection of LineStrings from track runs.
export function runsToGeoJSON(runs: TrackPoint[][]): FeatureCollection<LineString> {
  const features: Feature<LineString>[] = runs
    .filter((run) => run.length >= 2)
    .map((run, i) => ({
      type: 'Feature',
      id: i,
      geometry: {
        type: 'LineString',
        coordinates: run.map((p) => [p.lon, p.lat]),
      },
      properties: { run: i },
    }))
  return { type: 'FeatureCollection', features }
}

// Start/end points for all runs (for pin markers).
export function runEndpointsGeoJSON(runs: TrackPoint[][]): FeatureCollection<Point> {
  const features: Feature<Point>[] = []
  runs.forEach((run) => {
    if (run.length < 1) return
    features.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [run[0].lon, run[0].lat] },
      properties: { role: 'start' },
    })
    const last = run[run.length - 1]
    if (last !== run[0]) {
      features.push({
        type: 'Feature',
        geometry: { type: 'Point', coordinates: [last.lon, last.lat] },
        properties: { role: 'end' },
      })
    }
  })
  return { type: 'FeatureCollection', features }
}

// Build a GeoJSON FeatureCollection of Points from clip markers.
export function markersToGeoJSON(markers: ClipMarker[]): FeatureCollection<Point> {
  const features: Feature<Point>[] = markers.map((m) => ({
    type: 'Feature',
    id: m.id,
    geometry: { type: 'Point', coordinates: [m.lon, m.lat] },
    properties: {
      id: m.id,
      kind: m.kind,
      clip_ts: m.clip_ts,
      has_sei: m.has_sei,
    },
  }))
  return { type: 'FeatureCollection', features }
}

// Build a spider GeoJSON: n points offset in a circle around center.
export function spiderGeoJSON(
  center: [number, number],
  leaves: ClipMarker[],
  zoom: number,
): FeatureCollection<Point> {
  const count = leaves.length
  if (count === 0) return { type: 'FeatureCollection', features: [] }
  if (count === 1) {
    return markersToGeoJSON(leaves)
  }
  const radiusPx = 36
  const metersPerPx = (156543.03392 * Math.cos((center[1] * Math.PI) / 180)) / Math.pow(2, zoom)
  const dLng = ((radiusPx * metersPerPx) / 111320) * 1.2
  const dLat = ((radiusPx * metersPerPx) / 110574) * 1.2

  const features: Feature<Point>[] = leaves.map((leaf, i) => {
    const angle = (2 * Math.PI * i) / count - Math.PI / 2
    return {
      type: 'Feature',
      id: leaf.id,
      geometry: {
        type: 'Point',
        coordinates: [center[0] + dLng * Math.cos(angle), center[1] + dLat * Math.sin(angle)],
      },
      properties: {
        id: leaf.id,
        kind: leaf.kind,
        clip_ts: leaf.clip_ts,
        has_sei: leaf.has_sei,
        spider: true,
        cx: center[0],
        cy: center[1],
      },
    }
  })
  return { type: 'FeatureCollection', features }
}

// Compute bounding box [[minLng, minLat], [maxLng, maxLat]] from runs + markers.
export function computeBounds(
  runs: TrackPoint[][],
  markers: ClipMarker[],
): [[number, number], [number, number]] | null {
  let minLng = Infinity,
    minLat = Infinity,
    maxLng = -Infinity,
    maxLat = -Infinity

  for (const run of runs) {
    for (const p of run) {
      if (p.lon < minLng) minLng = p.lon
      if (p.lat < minLat) minLat = p.lat
      if (p.lon > maxLng) maxLng = p.lon
      if (p.lat > maxLat) maxLat = p.lat
    }
  }
  for (const m of markers) {
    if (m.lon < minLng) minLng = m.lon
    if (m.lat < minLat) minLat = m.lat
    if (m.lon > maxLng) maxLng = m.lon
    if (m.lat > maxLat) maxLat = m.lat
  }
  if (!isFinite(minLng)) return null
  return [
    [minLng, minLat],
    [maxLng, maxLat],
  ]
}

// Project a point by distM metres along bearingDeg (clockwise from north).
// Uses equirectangular approximation — accurate for short distances (<few km).
export function offsetLatLon(
  lat: number,
  lon: number,
  distM: number,
  bearingDeg: number,
): [number, number] {
  const b = (bearingDeg * Math.PI) / 180
  const north = Math.cos(b)
  const east = Math.sin(b)
  const dLat = (north * distM) / 111320
  const dLon = (east * distM) / (111320 * Math.cos((lat * Math.PI) / 180))
  return [lat + dLat, lon + dLon]
}

// Quantile density bucketing for the calendar heatmap (pure, unit-tested).
export { densityThresholds, densityLevel } from '@/lib/datetime'
