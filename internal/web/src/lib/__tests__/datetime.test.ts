import { describe, it, expect } from 'vitest'
import { Temporal } from 'temporal-polyfill'
import {
  buildMonthGrid,
  densityThresholds,
  densityLevel,
  inRange,
  outOfRangeCap,
  rangeToUTC,
  monthBoundsUTC,
  formatLocalTime,
  formatLocalDate,
  MAX_RANGE_DAYS,
} from '../datetime'
import type { CalendarDay } from '@/types'

describe('buildMonthGrid', () => {
  it('starts on Monday for a month beginning on Monday', () => {
    // May 2026 starts on Friday (dayOfWeek=5)
    const ym = Temporal.PlainYearMonth.from('2026-05')
    const grid = buildMonthGrid(ym)
    // First 4 cells should be null (Mon-Thu), 5th should be May 1
    expect(grid[0]).toBeNull()
    expect(grid[4]).not.toBeNull()
    expect((grid[4] as Temporal.PlainDate).day).toBe(1)
  })

  it('has length divisible by 7', () => {
    for (const m of ['2026-01', '2026-02', '2026-03', '2026-04']) {
      const grid = buildMonthGrid(Temporal.PlainYearMonth.from(m))
      expect(grid.length % 7).toBe(0)
    }
  })

  it('contains all days of the month', () => {
    const ym = Temporal.PlainYearMonth.from('2026-05')
    const grid = buildMonthGrid(ym)
    const days = grid.filter((c) => c !== null).map((c) => (c as Temporal.PlainDate).day)
    expect(days.length).toBe(31)
    expect(days[0]).toBe(1)
    expect(days[days.length - 1]).toBe(31)
  })
})

describe('densityThresholds', () => {
  it('returns [1,2] for empty data', () => {
    expect(densityThresholds([])).toEqual([1, 2])
  })

  it('computes correct p33/p67', () => {
    const days: CalendarDay[] = [1, 2, 3, 4, 5, 6, 7, 8, 9].map((n) => ({
      date: `2026-05-0${n}`,
      total: n * 10,
      recent: 0,
      saved: 0,
      sentry: 0,
    }))
    const [p33, p67] = densityThresholds(days)
    expect(p33).toBeLessThan(p67)
    expect(p33).toBeGreaterThan(0)
  })
})

describe('densityLevel', () => {
  it('returns none for 0', () => expect(densityLevel(0, [3, 6])).toBe('none'))
  it('returns low below p33', () => expect(densityLevel(2, [3, 6])).toBe('low'))
  it('returns mid between thresholds', () => expect(densityLevel(4, [3, 6])).toBe('mid'))
  it('returns high at/above p67', () => expect(densityLevel(6, [3, 6])).toBe('high'))
})

describe('inRange', () => {
  const from = Temporal.PlainDate.from('2026-05-01')
  const to = Temporal.PlainDate.from('2026-05-07')
  it('returns true for a date inside', () => {
    expect(inRange(Temporal.PlainDate.from('2026-05-04'), from, to)).toBe(true)
  })
  it('includes endpoints', () => {
    expect(inRange(from, from, to)).toBe(true)
    expect(inRange(to, from, to)).toBe(true)
  })
  it('returns false outside', () => {
    expect(inRange(Temporal.PlainDate.from('2026-05-08'), from, to)).toBe(false)
  })
})

describe('outOfRangeCap', () => {
  const anchor = Temporal.PlainDate.from('2026-05-01')
  it('allows dates within MAX_RANGE_DAYS', () => {
    const d = anchor.add({ days: MAX_RANGE_DAYS })
    expect(outOfRangeCap(d, anchor)).toBe(false)
  })
  it('disables dates beyond MAX_RANGE_DAYS', () => {
    const d = anchor.add({ days: MAX_RANGE_DAYS + 1 })
    expect(outOfRangeCap(d, anchor)).toBe(true)
  })
})

describe('rangeToUTC', () => {
  it('converts a local date range to UTC RFC3339 bounds', () => {
    const from = Temporal.PlainDate.from('2026-05-01')
    const to = Temporal.PlainDate.from('2026-05-03')
    const { from: f, to: t } = rangeToUTC(from, to, 'UTC')
    // UTC: from = 2026-05-01T00:00:00Z, to = 2026-05-04T00:00:00Z (exclusive end = next day)
    expect(f).toContain('2026-05-01')
    expect(t).toContain('2026-05-04')
  })

  it('converts with a non-UTC zone', () => {
    const from = Temporal.PlainDate.from('2026-05-01')
    const to = Temporal.PlainDate.from('2026-05-01')
    const { from: f } = rangeToUTC(from, to, 'America/New_York')
    // New York is UTC-4 in May; midnight NY = 04:00 UTC
    expect(f).toContain('T04:00:00')
  })
})

describe('monthBoundsUTC', () => {
  it('spans the full month', () => {
    const ym = Temporal.PlainYearMonth.from('2026-05')
    const { from, to } = monthBoundsUTC(ym, 'UTC')
    expect(from).toContain('2026-05-01')
    expect(to).toContain('2026-06-01')
  })
})

describe('formatLocalTime', () => {
  it('formats a UTC timestamp as HH:MM', () => {
    const ts = '2026-05-27T14:02:00Z'
    expect(formatLocalTime(ts, 'UTC')).toBe('14:02')
  })
})

describe('formatLocalDate', () => {
  it('includes the weekday and month', () => {
    const ts = '2026-05-27T14:02:00Z'
    const result = formatLocalDate(ts, 'UTC')
    expect(result).toContain('Wed')
    expect(result).toContain('May')
    expect(result).toContain('27')
  })
})
