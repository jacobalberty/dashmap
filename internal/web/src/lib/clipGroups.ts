import type { ClipListItem } from '@/types'
import { clipLocalDate, formatDateHeader } from '@/lib/datetime'

// A single standalone clip (no event — e.g. RecentClips).
export interface ClipEntry {
  type: 'clip'
  clip: ClipListItem
}

// A Saved/Sentry event collapsing its consecutive 1-minute clips into one entry.
export interface EventEntry {
  type: 'event'
  eventId: number
  clips: ClipListItem[] // in received (newest-first) order
  reason: string | null
  kind: ClipListItem['kind']
  thumbUrl: string | null
  hasGps: boolean
  firstClip: ClipListItem // earliest by clip_ts — the playback entry point
  latestClip: ClipListItem // newest by clip_ts
  count: number
}

export type Entry = ClipEntry | EventEntry

export interface DateGroup {
  key: string
  label: string
  entries: Entry[]
}

// Group clips into per-day buckets, collapsing each event's clips into one EventEntry.
// Clips are expected newest-first; an event's clips are contiguous in that order.
// API timestamps are UTC RFC3339, so lexicographic string compare orders them correctly.
export function groupClips(clips: ClipListItem[], tz: string): DateGroup[] {
  const dateGroups: DateGroup[] = []
  const dateIndex = new Map<string, DateGroup>()
  const eventIndex = new Map<string, EventEntry>() // key: `${dateKey}:${eventId}`

  for (const clip of clips) {
    const d = clipLocalDate(clip.clip_ts, tz)
    const dateKey = d.toString()

    let dg = dateIndex.get(dateKey)
    if (!dg) {
      dg = { key: dateKey, label: formatDateHeader(d), entries: [] }
      dateIndex.set(dateKey, dg)
      dateGroups.push(dg)
    }

    if (clip.event_id == null) {
      dg.entries.push({ type: 'clip', clip })
      continue
    }

    const evKey = `${dateKey}:${clip.event_id}`
    let ev = eventIndex.get(evKey)
    if (!ev) {
      ev = {
        type: 'event',
        eventId: clip.event_id,
        clips: [],
        reason: clip.reason,
        kind: clip.kind,
        thumbUrl: clip.thumb_url,
        hasGps: false,
        firstClip: clip,
        latestClip: clip,
        count: 0,
      }
      eventIndex.set(evKey, ev)
      dg.entries.push(ev)
    }

    ev.clips.push(clip)
    ev.count = ev.clips.length
    if (clip.lat != null) ev.hasGps = true
    if (clip.clip_ts < ev.firstClip.clip_ts) ev.firstClip = clip
    if (clip.clip_ts > ev.latestClip.clip_ts) ev.latestClip = clip
    if (!ev.thumbUrl && clip.thumb_url) ev.thumbUrl = clip.thumb_url
    if (!ev.reason && clip.reason) ev.reason = clip.reason
  }

  return dateGroups
}
