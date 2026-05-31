import { describe, it, expect } from 'vitest'
import { runsToGeoJSON, runEndpointsGeoJSON, markersToGeoJSON, computeBounds, spiderGeoJSON } from '../geo'
import type { TrackPoint, ClipMarker } from '@/types'

const mkPt = (lat: number, lon: number, ts = '2026-05-01T12:00:00Z'): TrackPoint => ({
  ts,
  lat,
  lon,
  clip_id: 1,
})

const mkMarker = (id: number, lat: number, lon: number): ClipMarker => ({
  id,
  clip_ts: '2026-05-01T12:00:00Z',
  lat,
  lon,
  kind: 'recent',
  has_sei: false,
})

describe('runsToGeoJSON', () => {
  it('returns a FeatureCollection', () => {
    const runs = [[mkPt(37, -122), mkPt(38, -121)]]
    const fc = runsToGeoJSON(runs)
    expect(fc.type).toBe('FeatureCollection')
    expect(fc.features).toHaveLength(1)
    expect(fc.features[0].geometry.type).toBe('LineString')
  })

  it('coordinates are [lon, lat]', () => {
    const runs = [[mkPt(37, -122), mkPt(38, -121)]]
    const coords = (runsToGeoJSON(runs).features[0].geometry as GeoJSON.LineString).coordinates
    expect(coords[0]).toEqual([-122, 37])
    expect(coords[1]).toEqual([-121, 38])
  })

  it('skips runs with fewer than 2 points', () => {
    const runs = [[mkPt(37, -122)], [mkPt(37, -122), mkPt(38, -121)]]
    expect(runsToGeoJSON(runs).features).toHaveLength(1)
  })

  it('returns empty FeatureCollection for empty input', () => {
    expect(runsToGeoJSON([]).features).toHaveLength(0)
  })
})

describe('runEndpointsGeoJSON', () => {
  it('produces start and end points per run', () => {
    const runs = [[mkPt(37, -122), mkPt(38, -121), mkPt(39, -120)]]
    const fc = runEndpointsGeoJSON(runs)
    expect(fc.features).toHaveLength(2)
    expect(fc.features[0].properties?.role).toBe('start')
    expect(fc.features[1].properties?.role).toBe('end')
  })

  it('produces one point for a single-point run', () => {
    const runs = [[mkPt(37, -122)]]
    const fc = runEndpointsGeoJSON(runs)
    expect(fc.features).toHaveLength(1)
  })
})

describe('markersToGeoJSON', () => {
  it('converts markers to point features', () => {
    const markers = [mkMarker(1, 37, -122), mkMarker(2, 38, -121)]
    const fc = markersToGeoJSON(markers)
    expect(fc.features).toHaveLength(2)
    expect((fc.features[0].geometry as GeoJSON.Point).coordinates).toEqual([-122, 37])
    expect(fc.features[0].properties?.kind).toBe('recent')
    expect(fc.features[0].properties?.id).toBe(1)
  })
})

describe('spiderGeoJSON', () => {
  it('returns single point unchanged for 1 leaf', () => {
    const fc = spiderGeoJSON([-122, 37], [mkMarker(1, 37, -122)], 14)
    expect(fc.features).toHaveLength(1)
  })

  it('offsets multiple leaves in a circle', () => {
    const leaves = [mkMarker(1, 37, -122), mkMarker(2, 37, -122), mkMarker(3, 37, -122)]
    const fc = spiderGeoJSON([-122, 37], leaves, 14)
    expect(fc.features).toHaveLength(3)
    // All should be offset from center
    for (const f of fc.features) {
      const [lng, lat] = (f.geometry as GeoJSON.Point).coordinates
      const moved = lng !== -122 || lat !== 37
      expect(moved).toBe(true)
    }
  })

  it('returns empty for no leaves', () => {
    expect(spiderGeoJSON([-122, 37], [], 14).features).toHaveLength(0)
  })
})

describe('computeBounds', () => {
  it('computes correct bounding box', () => {
    const runs = [[mkPt(37, -122), mkPt(38, -121)]]
    const markers = [mkMarker(1, 36, -123)]
    const bounds = computeBounds(runs, markers)
    expect(bounds).not.toBeNull()
    expect(bounds![0]).toEqual([-123, 36])
    expect(bounds![1]).toEqual([-121, 38])
  })

  it('returns null for empty data', () => {
    expect(computeBounds([], [])).toBeNull()
  })
})
