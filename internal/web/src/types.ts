export interface Envelope<T> {
  data: T
  meta: unknown
  error: { code: string; message: string } | null
}

export interface Car {
  id: number
  key: string
  display_name: string | null
  hw_generation: string | null
}

export interface CalendarDay {
  date: string // YYYY-MM-DD
  total: number
  recent: number
  saved: number
  sentry: number
}

export interface ClipListItem {
  id: number
  car_id: number
  clip_ts: string // RFC3339
  kind: 'recent' | 'saved' | 'sentry'
  lat: number | null
  lon: number | null
  has_sei: boolean
  duration_s: number | null
  reason: string | null
  city: string | null
  thumb_url: string | null
  cameras: string[]
}

export interface ClipMarker {
  id: number
  clip_ts: string // RFC3339
  lat: number
  lon: number
  kind: 'recent' | 'saved' | 'sentry'
  has_sei: boolean
}

export interface TrackPoint {
  ts: string // RFC3339
  lat: number
  lon: number
  clip_id: number
}

export interface ClipSegment {
  segment_id: number
  camera: string
  media_url: string
  download_url: string
  has_sei: boolean
  size_bytes: number | null
}

export interface ClipEvent {
  event_ts: string // RFC3339
  reason: string | null
  city: string | null
  camera: string | null
}

export interface ClipDetail {
  id: number
  car_id: number
  clip_ts: string // RFC3339
  kind: 'recent' | 'saved' | 'sentry'
  lat: number | null
  lon: number | null
  has_sei: boolean
  duration_s: number | null
  thumb_url: string | null
  download_url: string
  event: ClipEvent | null
  segments: ClipSegment[]
}

export interface TrackMeta {
  simplified: boolean
  runs: number
  returned: number
  total: number
}

export interface ClipsMeta {
  limit: number
  offset: number
  has_more: boolean
}

export interface MarkersMeta {
  capped: boolean
}

export interface StatusCounts {
  cars: number
  events: number
  clips: number
  segments: number
  track_points: number
}

export interface CarStatus {
  car_id: number
  key: string
  last_scan_started_at: string | null
  last_scan_completed_at: string | null
  live: boolean
  last_scan: { processed: number; skipped: number; errored: number; track_points: number }
}

export interface Status {
  db_reachable: boolean
  counts: StatusCounts
  car: CarStatus
}
