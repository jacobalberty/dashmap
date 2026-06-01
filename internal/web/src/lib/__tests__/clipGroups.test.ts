import { describe, it, expect } from 'vitest'
import { groupClips, type EventEntry } from '../clipGroups'
import type { ClipListItem } from '@/types'

function mkClip(overrides: Partial<ClipListItem> & { id: number; clip_ts: string }): ClipListItem {
  return {
    car_id: 1,
    kind: 'sentry',
    lat: null,
    lon: null,
    has_sei: false,
    duration_s: 60,
    reason: 'sentry_aware_object_detection',
    city: null,
    thumb_url: '/api/thumb/1',
    event_id: null,
    cameras: ['front'],
    ...overrides,
  }
}

describe('groupClips', () => {
  it('collapses consecutive clips of one event into a single entry', () => {
    // newest-first, as the API returns
    const clips = [
      mkClip({ id: 13, clip_ts: '2026-04-18T17:14:23Z', event_id: 1366 }),
      mkClip({ id: 12, clip_ts: '2026-04-18T17:13:22Z', event_id: 1366 }),
      mkClip({ id: 11, clip_ts: '2026-04-18T17:03:20Z', event_id: 1366 }),
    ]
    const groups = groupClips(clips, 'UTC')
    expect(groups).toHaveLength(1)
    expect(groups[0].entries).toHaveLength(1)
    const ev = groups[0].entries[0] as EventEntry
    expect(ev.type).toBe('event')
    expect(ev.eventId).toBe(1366)
    expect(ev.count).toBe(3)
    // earliest clip is the playback entry point
    expect(ev.firstClip.id).toBe(11)
    expect(ev.latestClip.id).toBe(13)
  })

  it('keeps standalone clips (event_id null) as individual entries', () => {
    const clips = [
      mkClip({ id: 2, clip_ts: '2026-04-18T10:02:00Z', event_id: null, kind: 'recent', reason: null }),
      mkClip({ id: 1, clip_ts: '2026-04-18T10:01:00Z', event_id: null, kind: 'recent', reason: null }),
    ]
    const groups = groupClips(clips, 'UTC')
    expect(groups[0].entries).toHaveLength(2)
    expect(groups[0].entries.every((e) => e.type === 'clip')).toBe(true)
  })

  it('does not merge two different events', () => {
    const clips = [
      mkClip({ id: 20, clip_ts: '2026-04-18T18:00:00Z', event_id: 200 }),
      mkClip({ id: 10, clip_ts: '2026-04-18T17:00:00Z', event_id: 100 }),
    ]
    const groups = groupClips(clips, 'UTC')
    expect(groups[0].entries).toHaveLength(2)
    const ids = groups[0].entries.map((e) => (e as EventEntry).eventId)
    expect(ids).toEqual([200, 100])
  })

  it('marks hasGps true when any clip has coordinates', () => {
    const clips = [
      mkClip({ id: 2, clip_ts: '2026-04-18T17:02:00Z', event_id: 5, lat: null }),
      mkClip({ id: 1, clip_ts: '2026-04-18T17:01:00Z', event_id: 5, lat: 37.7, lon: -122.4 }),
    ]
    const ev = groupClips(clips, 'UTC')[0].entries[0] as EventEntry
    expect(ev.hasGps).toBe(true)
  })

  it('splits clips across separate local dates', () => {
    const clips = [
      mkClip({ id: 2, clip_ts: '2026-04-19T00:30:00Z', event_id: null, kind: 'recent', reason: null }),
      mkClip({ id: 1, clip_ts: '2026-04-18T23:30:00Z', event_id: null, kind: 'recent', reason: null }),
    ]
    const groups = groupClips(clips, 'UTC')
    expect(groups).toHaveLength(2)
    expect(groups[0].key).toBe('2026-04-19')
    expect(groups[1].key).toBe('2026-04-18')
  })

  it('interleaves standalone clips and events in received order', () => {
    const clips = [
      mkClip({ id: 30, clip_ts: '2026-04-18T18:00:00Z', event_id: null, kind: 'recent', reason: null }),
      mkClip({ id: 21, clip_ts: '2026-04-18T17:05:00Z', event_id: 100 }),
      mkClip({ id: 20, clip_ts: '2026-04-18T17:04:00Z', event_id: 100 }),
    ]
    const entries = groupClips(clips, 'UTC')[0].entries
    expect(entries).toHaveLength(2)
    expect(entries[0].type).toBe('clip')
    expect(entries[1].type).toBe('event')
    expect((entries[1] as EventEntry).count).toBe(2)
  })
})
