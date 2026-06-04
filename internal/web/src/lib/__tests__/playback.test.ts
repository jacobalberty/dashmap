import { describe, it, expect } from 'vitest'
import {
  backendTelemetry,
  extendTrack,
  formatGear,
  formatAutopilot,
  formatSpeedMph,
  formatSpeedKmh,
  headingCardinal,
  shouldResync,
} from '../playback'
import type { ClipTrackPoint } from '@/types'

const BASE_TS = new Date('2026-05-30T12:00:00.000Z').getTime()

function pt(offsetSec: number, overrides: Partial<ClipTrackPoint> = {}): ClipTrackPoint {
  return {
    ts: new Date(BASE_TS + offsetSec * 1000).toISOString(),
    lat: 37.77 + offsetSec * 0.001,
    lon: -122.41,
    speed_mps: 10,
    heading_deg: 0,
    gear: 1,
    autopilot: 0,
    blinker_left: false,
    blinker_right: false,
    brake_applied: false,
    accelerator_pedal_position: 0.5,
    steering_wheel_angle: 0,
    accel_x: null,
    accel_y: null,
    accel_z: null,
    sei_version: null,
    ...overrides,
  }
}

describe('backendTelemetry — empty', () => {
  it('returns null for any time', () => {
    const src = backendTelemetry(BASE_TS, [])
    expect(src.at(0)).toBeNull()
    expect(src.hasGps).toBe(false)
  })
})

describe('backendTelemetry — single point', () => {
  it('returns that point for any time', () => {
    const src = backendTelemetry(BASE_TS, [pt(5, { speed_mps: 20 })])
    expect(src.at(0)?.speedMps).toBe(20)
    expect(src.at(10)?.speedMps).toBe(20)
    expect(src.hasGps).toBe(true)
  })
})

describe('backendTelemetry — interpolation', () => {
  const pts = [pt(0, { lat: 37.0, speed_mps: 0, heading_deg: 0 }), pt(10, { lat: 37.1, speed_mps: 20, heading_deg: 0 })]
  const src = backendTelemetry(BASE_TS, pts)

  it('clamps before first point', () => {
    const t = src.at(-5)
    expect(t?.lat).toBeCloseTo(37.0, 5)
    expect(t?.speedMps).toBe(0)
  })

  it('clamps after last point', () => {
    const t = src.at(100)
    expect(t?.lat).toBeCloseTo(37.1, 4)
    expect(t?.speedMps).toBe(20)
  })

  it('linearly interpolates at midpoint', () => {
    const t = src.at(5)
    expect(t?.lat).toBeCloseTo(37.05, 5)
    expect(t?.speedMps).toBeCloseTo(10, 5)
  })

  it('returns exact start value at t=0', () => {
    expect(src.at(0)?.lat).toBeCloseTo(37.0, 5)
  })

  it('returns exact end value at t=10', () => {
    expect(src.at(10)?.lat).toBeCloseTo(37.1, 4)
  })
})

describe('backendTelemetry — discrete fields step at lo', () => {
  const pts = [
    pt(0, { gear: 0, autopilot: 0, blinker_left: false }),
    pt(10, { gear: 1, autopilot: 2, blinker_left: true }),
  ]
  const src = backendTelemetry(BASE_TS, pts)

  it('uses lo gear at midpoint', () => {
    expect(src.at(5)?.gear).toBe(0)
  })

  it('uses hi gear exactly at hi timestamp', () => {
    expect(src.at(10)?.gear).toBe(1)
  })

  it('uses lo blinker at midpoint', () => {
    expect(src.at(5)?.blinkerLeft).toBe(false)
  })
})

describe('backendTelemetry — heading shortest-arc lerp', () => {
  it('interpolates 350°→10° through 0° not 180°', () => {
    const pts = [
      pt(0, { heading_deg: 350, lat: 37.0, speed_mps: 0 }),
      pt(10, { heading_deg: 10, lat: 37.1, speed_mps: 0 }),
    ]
    const src = backendTelemetry(BASE_TS, pts)
    const mid = src.at(5)?.headingDeg ?? 180
    // Should be 0° (or very close), not 180°
    expect(mid).toBeCloseTo(0, 0)
  })

  it('interpolates 10°→350° through 0°', () => {
    const pts = [
      pt(0, { heading_deg: 10, lat: 37.0, speed_mps: 0 }),
      pt(10, { heading_deg: 350, lat: 37.1, speed_mps: 0 }),
    ]
    const src = backendTelemetry(BASE_TS, pts)
    const mid = src.at(5)?.headingDeg ?? 180
    // Should wrap around 0 (either ~0° or ~360°)
    const normalised = ((mid % 360) + 360) % 360
    expect(normalised).toBeCloseTo(0, 0)
  })
})

describe('backendTelemetry — nullable fields', () => {
  it('returns null accel when both endpoints are null', () => {
    const pts = [pt(0, { accel_x: null }), pt(10, { accel_x: null })]
    const src = backendTelemetry(BASE_TS, pts)
    expect(src.at(5)?.accelX).toBeNull()
  })

  it('returns null accel when one endpoint is null', () => {
    const pts = [pt(0, { accel_x: 1.0 }), pt(10, { accel_x: null })]
    const src = backendTelemetry(BASE_TS, pts)
    expect(src.at(5)?.accelX).toBeNull()
  })
})

describe('backendTelemetry — timeAt (inverse)', () => {
  const pts = [pt(0, { lat: 37.0, lon: -122.0 }), pt(10, { lat: 37.1, lon: -122.0 })]
  const src = backendTelemetry(BASE_TS, pts)

  it('projects a midpoint to the mid time', () => {
    expect(src.timeAt(-122.0, 37.05)).toBeCloseTo(5, 1)
  })

  it('projects an off-path point to the nearest point time', () => {
    // east of the line at the midpoint latitude → still projects to mid time
    expect(src.timeAt(-121.999, 37.05)).toBeCloseTo(5, 1)
  })

  it('clamps before the first point', () => {
    expect(src.timeAt(-122.0, 36.9)).toBeCloseTo(0, 5)
  })

  it('clamps after the last point', () => {
    expect(src.timeAt(-122.0, 37.2)).toBeCloseTo(10, 5)
  })

  it('returns the single point time regardless of query', () => {
    const one = backendTelemetry(BASE_TS, [pt(7, { lat: 37.0, lon: -122.0 })])
    expect(one.timeAt(-100, 10)).toBe(7)
  })

  it('returns null for an empty track', () => {
    expect(backendTelemetry(BASE_TS, []).timeAt(-122, 37)).toBeNull()
  })
})

describe('formatGear', () => {
  it('maps 0→P 1→D 2→R 3→N', () => {
    expect(formatGear(0)).toBe('P')
    expect(formatGear(1)).toBe('D')
    expect(formatGear(2)).toBe('R')
    expect(formatGear(3)).toBe('N')
  })
  it('returns ? for unknown', () => {
    expect(formatGear(99)).toBe('?')
  })
})

describe('formatAutopilot', () => {
  it('maps 0→— 1→FSD 2→Autosteer 3→TACC', () => {
    expect(formatAutopilot(0)).toBe('—')
    expect(formatAutopilot(1)).toBe('FSD')
    expect(formatAutopilot(2)).toBe('Autosteer')
    expect(formatAutopilot(3)).toBe('TACC')
  })
})

describe('speed formatters', () => {
  it('converts 10 m/s to ~22 mph', () => {
    expect(formatSpeedMph(10)).toBe(22)
  })
  it('converts 10 m/s to 36 km/h', () => {
    expect(formatSpeedKmh(10)).toBe(36)
  })
})

describe('headingCardinal', () => {
  it('maps 0→N 90→E 180→S 270→W', () => {
    expect(headingCardinal(0)).toBe('N')
    expect(headingCardinal(90)).toBe('E')
    expect(headingCardinal(180)).toBe('S')
    expect(headingCardinal(270)).toBe('W')
  })
  it('handles wrap around 360', () => {
    expect(headingCardinal(360)).toBe('N')
    expect(headingCardinal(359)).toBe('N')
  })
})

describe('shouldResync', () => {
  it('returns false when within threshold', () => {
    expect(shouldResync(1.0, 1.05, 0.1)).toBe(false)
  })
  it('returns true when outside threshold', () => {
    expect(shouldResync(1.0, 1.2, 0.1)).toBe(true)
  })
  it('uses default threshold of 0.1', () => {
    expect(shouldResync(0, 0.09)).toBe(false)
    expect(shouldResync(0, 0.11)).toBe(true)
  })
})

describe('extendTrack', () => {
  it('returns empty for empty raw', () => {
    const r = extendTrack([], BASE_TS, 60)
    expect(r.points).toHaveLength(0)
    expect(r.headStub).toBeNull()
    expect(r.tailStub).toBeNull()
  })

  it('returns no stubs when durationSec<=0', () => {
    const r = extendTrack([pt(5)], BASE_TS, 0)
    expect(r.points).toHaveLength(1)
    expect(r.headStub).toBeNull()
    expect(r.tailStub).toBeNull()
  })

  it('returns no stubs when edges are already covered', () => {
    // first fix at t=0, last at t=10 → no gap
    const r = extendTrack([pt(0), pt(10)], BASE_TS, 10)
    expect(r.points).toHaveLength(2)
    expect(r.headStub).toBeNull()
    expect(r.tailStub).toBeNull()
  })

  it('adds head + tail stubs for a moving car with gaps', () => {
    // first fix at t=5 (gap=5s head), last at t=25 (gap=5s tail), duration=30
    const raw = [
      pt(5,  { lat: 37.0, lon: -122.0, speed_mps: 20, heading_deg: 0 }),
      pt(25, { lat: 37.1, lon: -122.0, speed_mps: 20, heading_deg: 0 }),
    ]
    const r = extendTrack(raw, BASE_TS, 30)
    // synthetic head at t=0, synthetic tail at t=30
    expect(r.points[0].ts).toBe(new Date(BASE_TS).toISOString())
    expect(r.points[r.points.length - 1].ts).toBe(new Date(BASE_TS + 30000).toISOString())
    // stubs should be non-null and have 2 coordinate pairs
    expect(r.headStub).not.toBeNull()
    expect(r.tailStub).not.toBeNull()
    expect(r.headStub).toHaveLength(2)
    expect(r.tailStub).toHaveLength(2)
    // heading 0° = north; backward = south → head lat < first lat
    expect(r.points[0].lat).toBeLessThan(37.0)
    // forward = north → tail lat > last lat
    expect(r.points[r.points.length - 1].lat).toBeGreaterThan(37.1)
  })

  it('caps stub distance at STUB_CAP_S * speed', () => {
    // 10s gap at 20 m/s → uncapped = 200m, capped = 2s * 20 = 40m
    const raw = [pt(10, { lat: 37.0, lon: -122.0, speed_mps: 20, heading_deg: 0 })]
    const r = extendTrack(raw, BASE_TS, 60)
    const headLat = r.points[0].lat
    // 40m south (heading+180=south) ≈ 40/111320 degrees ≈ 0.000359°
    const expectedDist = 40 / 111320
    expect(Math.abs(37.0 - headLat)).toBeCloseTo(expectedDist, 4)
  })

  it('holds position when speed is zero', () => {
    const raw = [pt(5, { lat: 37.0, lon: -122.0, speed_mps: 0, heading_deg: 90 })]
    const r = extendTrack(raw, BASE_TS, 30)
    // Synthetic head and tail exist in points (for timeline coverage) but no stub geometry
    expect(r.points[0].lat).toBeCloseTo(37.0, 6)
    expect(r.points[0].lon).toBeCloseTo(-122.0, 6)
    expect(r.headStub).toBeNull()
    expect(r.tailStub).toBeNull()
  })

  it('holds position when heading is null', () => {
    const raw = [pt(5, { lat: 37.0, lon: -122.0, speed_mps: 20, heading_deg: null as unknown as number })]
    const r = extendTrack(raw, BASE_TS, 30)
    expect(r.points[0].lat).toBeCloseTo(37.0, 6)
    expect(r.headStub).toBeNull()
  })

  it('integration: at(0) and at(duration) return non-null after extending', () => {
    const raw = [
      pt(5,  { lat: 37.0, lon: -122.0, speed_mps: 10, heading_deg: 0 }),
      pt(25, { lat: 37.1, lon: -122.0, speed_mps: 10, heading_deg: 0 }),
    ]
    const { points } = extendTrack(raw, BASE_TS, 30)
    const src = backendTelemetry(BASE_TS, points)
    expect(src.at(0)).not.toBeNull()
    expect(src.at(30)).not.toBeNull()
  })
})
