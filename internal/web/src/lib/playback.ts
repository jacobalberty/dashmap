import type { ClipTrackPoint } from '@/types'

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

  return { at, hasGps }
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
