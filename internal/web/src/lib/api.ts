import type {
  Envelope,
  Car,
  CalendarDay,
  ClipListItem,
  ClipMarker,
  TrackPoint,
  ClipDetail,
  ClipTrackPoint,
  Status,
  ClipsMeta,
  MarkersMeta,
  TrackMeta,
} from '@/types'

class APIError extends Error {
  constructor(
    public code: string,
    message: string,
  ) {
    super(message)
    this.name = 'APIError'
  }
}

async function apiGet<T>(path: string, params?: Record<string, string | undefined>): Promise<T> {
  const url = new URL(path, location.origin)
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== '') url.searchParams.set(k, v)
    }
  }
  const res = await fetch(url.toString())
  const env: Envelope<T> = await res.json()
  if (env.error) throw new APIError(env.error.code, env.error.message)
  return env.data
}

async function apiGetWithMeta<T, M>(
  path: string,
  params?: Record<string, string | undefined>,
): Promise<{ data: T; meta: M }> {
  const url = new URL(path, location.origin)
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== '') url.searchParams.set(k, v)
    }
  }
  const res = await fetch(url.toString())
  const env: Envelope<T> & { meta: M } = await res.json()
  if (env.error) throw new APIError(env.error.code, env.error.message)
  return { data: env.data, meta: env.meta }
}

export function getCars(): Promise<Car[]> {
  return apiGet<Car[]>('/api/cars')
}

export function getCalendar(params: {
  from?: string
  to?: string
  tz?: string
  car?: string
}): Promise<CalendarDay[]> {
  return apiGet<CalendarDay[]>('/api/calendar', {
    from: params.from,
    to: params.to,
    tz: params.tz,
    car: params.car,
  })
}

export function getClips(params: {
  from: string
  to: string
  car?: string
  kind?: string
  limit?: number
  offset?: number
}): Promise<{ data: ClipListItem[]; meta: ClipsMeta }> {
  return apiGetWithMeta<ClipListItem[], ClipsMeta>('/api/clips', {
    from: params.from,
    to: params.to,
    car: params.car,
    kind: params.kind,
    limit: params.limit?.toString(),
    offset: params.offset?.toString(),
  })
}

export function getClipMarkers(params: {
  from: string
  to: string
  car?: string
}): Promise<{ data: ClipMarker[]; meta: MarkersMeta }> {
  return apiGetWithMeta<ClipMarker[], MarkersMeta>('/api/clip-markers', {
    from: params.from,
    to: params.to,
    car: params.car,
  })
}

export function getTrack(params: {
  from: string
  to: string
  car?: string
}): Promise<{ data: TrackPoint[][]; meta: TrackMeta }> {
  return apiGetWithMeta<TrackPoint[][], TrackMeta>('/api/track', {
    from: params.from,
    to: params.to,
    car: params.car,
  })
}

export function getClipDetail(id: number): Promise<ClipDetail> {
  return apiGet<ClipDetail>(`/api/clips/${id}`)
}

export function getClipTrack(id: number): Promise<ClipTrackPoint[]> {
  return apiGet<ClipTrackPoint[]>(`/api/clips/${id}/track`)
}

export function getStatus(car?: string): Promise<Status> {
  return apiGet<Status>('/api/status', { car })
}

export { APIError }
