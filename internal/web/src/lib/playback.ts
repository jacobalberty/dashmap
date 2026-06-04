import type { ClipTrackPoint } from '@/types'
import { offsetLatLon } from '@/lib/geo'

// ── Public types ──────────────────────────────────────────────────────────────

export interface Telemetry {
  lat: number
  lon: number
  speedMps: number
  headingDeg: number
  gear: number       // 0=P 1=D 2=R 3=N
  autopilot: number  // 0=none 1=FSD 2=Autosteer 3=TACC
  blinkerLeft: boolean
  blinkerRight: boolean
  brakeApplied: boolean
  steeringWheelAngle: number
  acceleratorPedalPosition: number | null
  accelX: number | null
  accelY: number | null
  accelZ: number | null
}

// TelemetrySource is the seam between backend-stored telemetry (this impl)
// and a future client-side per-frame SEI parse (via dashcam-mp4.js).
// Any implementation of this interface can be swapped in without changing views.
export interface TelemetrySource {
  at(tSec: number): Telemetry | null
  // Inverse of at(): given a geographic point, return the time of the closest
  // point on the track (clamped to the GPS-covered span), or null if no track.
  timeAt(lng: number, lat: number): number | null
  readonly hasGps: boolean
}

// ── backendTelemetry ──────────────────────────────────────────────────────────

// Pre-processed point for fast binary search.
interface PreparedPoint {
  tSec: number
  lat: number
  lon: number
  speedMps: number
  headingDeg: number
  gear: number
  autopilot: number
  blinkerLeft: boolean
  blinkerRight: boolean
  brakeApplied: boolean
  steeringWheelAngle: number
  acceleratorPedalPosition: number | null
  accelX: number | null
  accelY: number | null
  accelZ: number | null
}

function preparePoints(clipTsMs: number, raw: ClipTrackPoint[]): PreparedPoint[] {
  return raw.map((p) => ({
    tSec: (new Date(p.ts).getTime() - clipTsMs) / 1000,
    lat: p.lat,
    lon: p.lon,
    speedMps: p.speed_mps ?? 0,
    headingDeg: p.heading_deg ?? 0,
    gear: p.gear ?? 0,
    autopilot: p.autopilot ?? 0,
    blinkerLeft: p.blinker_left ?? false,
    blinkerRight: p.blinker_right ?? false,
    brakeApplied: p.brake_applied ?? false,
    steeringWheelAngle: p.steering_wheel_angle ?? 0,
    acceleratorPedalPosition: p.accelerator_pedal_position,
    accelX: p.accel_x,
    accelY: p.accel_y,
    accelZ: p.accel_z,
  }))
}

// Returns the index of the largest element ≤ target, or 0 if all are greater.
function bsearchLE(pts: PreparedPoint[], target: number): number {
  let lo = 0,
    hi = pts.length - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (pts[mid].tSec <= target) lo = mid
    else hi = mid - 1
  }
  return lo
}

function lerpNum(a: number, b: number, t: number): number {
  return a + (b - a) * t
}

function lerpHeading(a: number, b: number, t: number): number {
  // Shortest-arc interpolation so 350°→10° goes through 0° not 180°.
  const diff = (((b - a + 180) % 360) + 360) % 360 - 180
  return ((a + diff * t) % 360 + 360) % 360
}

function lerpNullable(
  a: number | null,
  b: number | null,
  t: number,
): number | null {
  if (a === null || b === null) return null
  return lerpNum(a, b, t)
}

function interpolate(lo: PreparedPoint, hi: PreparedPoint, t: number): Telemetry {
  return {
    lat: lerpNum(lo.lat, hi.lat, t),
    lon: lerpNum(lo.lon, hi.lon, t),
    speedMps: lerpNum(lo.speedMps, hi.speedMps, t),
    headingDeg: lerpHeading(lo.headingDeg, hi.headingDeg, t),
    // Discrete fields: use lo's value (current state at that moment).
    gear: lo.gear,
    autopilot: lo.autopilot,
    blinkerLeft: lo.blinkerLeft,
    blinkerRight: lo.blinkerRight,
    brakeApplied: lo.brakeApplied,
    steeringWheelAngle: lerpNum(lo.steeringWheelAngle, hi.steeringWheelAngle, t),
    acceleratorPedalPosition: lerpNullable(
      lo.acceleratorPedalPosition,
      hi.acceleratorPedalPosition,
      t,
    ),
    accelX: lerpNullable(lo.accelX, hi.accelX, t),
    accelY: lerpNullable(lo.accelY, hi.accelY, t),
    accelZ: lerpNullable(lo.accelZ, hi.accelZ, t),
  }
}

function pointToTelemetry(p: PreparedPoint): Telemetry {
  return {
    lat: p.lat,
    lon: p.lon,
    speedMps: p.speedMps,
    headingDeg: p.headingDeg,
    gear: p.gear,
    autopilot: p.autopilot,
    blinkerLeft: p.blinkerLeft,
    blinkerRight: p.blinkerRight,
    brakeApplied: p.brakeApplied,
    steeringWheelAngle: p.steeringWheelAngle,
    acceleratorPedalPosition: p.acceleratorPedalPosition,
    accelX: p.accelX,
    accelY: p.accelY,
    accelZ: p.accelZ,
  }
}

export function backendTelemetry(
  clipTsMs: number,
  raw: ClipTrackPoint[],
): TelemetrySource {
  const pts = preparePoints(clipTsMs, raw)
  const hasGps = pts.length > 0

  function at(tSec: number): Telemetry | null {
    if (pts.length === 0) return null
    if (pts.length === 1) return pointToTelemetry(pts[0])

    // Clamp to data range.
    if (tSec <= pts[0].tSec) return pointToTelemetry(pts[0])
    if (tSec >= pts[pts.length - 1].tSec) return pointToTelemetry(pts[pts.length - 1])

    const i = bsearchLE(pts, tSec)
    const lo = pts[i]
    const hi = pts[i + 1]
    const span = hi.tSec - lo.tSec
    const t = span > 0 ? (tSec - lo.tSec) / span : 0
    return interpolate(lo, hi, t)
  }

  // Project (lng,lat) onto the track polyline and return the time of the closest
  // point. Distances use a cos(lat)-scaled lon (equirectangular) — accurate
  // enough for a ~1-minute clip's small footprint.
  function timeAt(lng: number, lat: number): number | null {
    if (pts.length === 0) return null
    if (pts.length === 1) return pts[0].tSec

    const kx = Math.cos((lat * Math.PI) / 180) // lon → x scale at this latitude
    const px = lng * kx
    const py = lat

    let bestD2 = Infinity
    let bestT = pts[0].tSec
    for (let i = 0; i < pts.length - 1; i++) {
      const a = pts[i]
      const b = pts[i + 1]
      const ax = a.lon * kx
      const ay = a.lat
      const bx = b.lon * kx
      const by = b.lat
      const dx = bx - ax
      const dy = by - ay
      const len2 = dx * dx + dy * dy
      let u = len2 > 0 ? ((px - ax) * dx + (py - ay) * dy) / len2 : 0
      u = Math.max(0, Math.min(1, u))
      const cx = ax + u * dx
      const cy = ay + u * dy
      const ddx = px - cx
      const ddy = py - cy
      const d2 = ddx * ddx + ddy * ddy
      if (d2 < bestD2) {
        bestD2 = d2
        bestT = lerpNum(a.tSec, b.tSec, u)
      }
    }
    return bestT
  }

  return { at, timeAt, hasGps }
}

// ── extendTrack ───────────────────────────────────────────────────────────────

// Max seconds of travel to project for a head/tail stub.
const STUB_CAP_S = 2
// Below this speed (m/s) we hold position instead of projecting.
const STUB_MIN_SPEED = 0.5
// Treat gap as "edge already covered" if smaller than this (s).
const STUB_EPS = 0.01

export interface ExtendedTrack {
  points: ClipTrackPoint[]
  // [lon,lat] pairs for dashed stub line geometry, or null if no stub needed.
  headStub: [number, number][] | null
  tailStub: [number, number][] | null
}

// Prepend/append synthetic edge points so backendTelemetry covers the full clip
// span [0, durationSec], not just the GPS-valid span. Head and tail stubs
// dead-reckon from heading+speed at the edge fix, capped to STUB_CAP_S seconds
// of travel. When the car is stationary or durationSec is not yet known the
// synthetic point holds the edge fix's position (no visual stub geometry).
export function extendTrack(
  raw: ClipTrackPoint[],
  clipTsMs: number,
  durationSec: number,
): ExtendedTrack {
  if (raw.length === 0) return { points: [], headStub: null, tailStub: null }
  if (durationSec <= 0) return { points: raw, headStub: null, tailStub: null }

  const points: ClipTrackPoint[] = [...raw]
  let headStub: [number, number][] | null = null
  let tailStub: [number, number][] | null = null

  const first = raw[0]
  const last = raw[raw.length - 1]
  const firstTSec = (new Date(first.ts).getTime() - clipTsMs) / 1000
  const lastTSec = (new Date(last.ts).getTime() - clipTsMs) / 1000

  // Head stub: synthesize a point at t=0 projecting backward from the first fix.
  if (firstTSec > STUB_EPS) {
    const ts0 = new Date(clipTsMs).toISOString()
    const speed = first.speed_mps ?? 0
    const heading = first.heading_deg
    if (speed >= STUB_MIN_SPEED && heading != null) {
      const distM = speed * Math.min(firstTSec, STUB_CAP_S)
      const [newLat, newLon] = offsetLatLon(first.lat, first.lon, distM, heading + 180)
      points.unshift({ ...first, ts: ts0, lat: newLat, lon: newLon })
      headStub = [[newLon, newLat], [first.lon, first.lat]]
    } else {
      points.unshift({ ...first, ts: ts0 })
    }
  }

  // Tail stub: synthesize a point at t=duration projecting forward from the last fix.
  if (lastTSec < durationSec - STUB_EPS) {
    const tsEnd = new Date(clipTsMs + durationSec * 1000).toISOString()
    const speed = last.speed_mps ?? 0
    const heading = last.heading_deg
    if (speed >= STUB_MIN_SPEED && heading != null) {
      const distM = speed * Math.min(durationSec - lastTSec, STUB_CAP_S)
      const [newLat, newLon] = offsetLatLon(last.lat, last.lon, distM, heading)
      points.push({ ...last, ts: tsEnd, lat: newLat, lon: newLon })
      tailStub = [[last.lon, last.lat], [newLon, newLat]]
    } else {
      points.push({ ...last, ts: tsEnd })
    }
  }

  return { points, headStub, tailStub }
}

// ── Formatters ────────────────────────────────────────────────────────────────

const GEAR_LABELS = ['P', 'D', 'R', 'N']
export function formatGear(g: number): string {
  return GEAR_LABELS[g] ?? '?'
}

const AUTOPILOT_LABELS = ['—', 'FSD', 'Autosteer', 'TACC']
export function formatAutopilot(a: number): string {
  return AUTOPILOT_LABELS[a] ?? '?'
}

export function formatSpeedMph(mps: number): number {
  return Math.round(mps * 2.23694)
}

export function formatSpeedKmh(mps: number): number {
  return Math.round(mps * 3.6)
}

const CARDINALS = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW']
export function headingCardinal(deg: number): string {
  return CARDINALS[Math.round((((deg % 360) + 360) % 360) / 45) % 8]
}

export function shouldResync(masterT: number, slaveT: number, threshold = 0.1): boolean {
  return Math.abs(masterT - slaveT) > threshold
}
