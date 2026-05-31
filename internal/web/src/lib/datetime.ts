import { Temporal } from 'temporal-polyfill'
import type { CalendarDay } from '@/types'

export { Temporal }

export const MAX_RANGE_DAYS = 31

export function localTz(): string {
  return Temporal.Now.timeZoneId()
}

// Today in the local zone as a PlainDate.
export function today(): Temporal.PlainDate {
  return Temporal.Now.plainDateISO()
}

// Current year-month in the local zone.
export function currentMonth(): Temporal.PlainYearMonth {
  return today().toPlainYearMonth()
}

// Convert a PlainDate (local) to UTC RFC3339 midnight (start of that local day).
export function plainDateToUTCStart(d: Temporal.PlainDate, tz: string): string {
  const zdt = d.toZonedDateTime({ timeZone: tz, plainTime: Temporal.PlainTime.from('00:00:00') })
  return zdt.toInstant().toString()
}

// Convert a PlainDate (local) to UTC RFC3339: exclusive end of that local day (next day midnight).
export function plainDateToUTCEnd(d: Temporal.PlainDate, tz: string): string {
  const next = d.add({ days: 1 })
  return plainDateToUTCStart(next, tz)
}

// Build the [from, to) UTC bounds for a local-date range [start, end] (inclusive both ends).
export function rangeToUTC(
  from: Temporal.PlainDate,
  to: Temporal.PlainDate,
  tz: string,
): { from: string; to: string } {
  return {
    from: plainDateToUTCStart(from, tz),
    to: plainDateToUTCEnd(to, tz),
  }
}

// Month bounds [first-of-month, first-of-next-month) as local PlainDates.
export function monthBounds(ym: Temporal.PlainYearMonth): {
  from: Temporal.PlainDate
  to: Temporal.PlainDate
} {
  const from = ym.toPlainDate({ day: 1 })
  const to = from.add({ months: 1 })
  return { from, to }
}

// Month bounds as UTC RFC3339 strings (for the calendar API call).
export function monthBoundsUTC(
  ym: Temporal.PlainYearMonth,
  tz: string,
): { from: string; to: string } {
  const { from, to } = monthBounds(ym)
  return {
    from: plainDateToUTCStart(from, tz),
    to: plainDateToUTCStart(to, tz),
  }
}

// Build the 7-col grid for a month. Cells are PlainDates or null (padding).
export function buildMonthGrid(ym: Temporal.PlainYearMonth): (Temporal.PlainDate | null)[] {
  const first = ym.toPlainDate({ day: 1 })
  const days = ym.daysInMonth
  // dayOfWeek: 1=Mon … 7=Sun
  const offset = first.dayOfWeek - 1
  const cells: (Temporal.PlainDate | null)[] = Array(offset).fill(null)
  for (let d = 1; d <= days; d++) {
    cells.push(ym.toPlainDate({ day: d }))
  }
  while (cells.length % 7 !== 0) cells.push(null)
  return cells
}

// Density thresholds: returns [p33, p67] of non-zero totals for quantile bucketing.
export function densityThresholds(days: CalendarDay[]): [number, number] {
  const nonZero = days
    .map((d) => d.total)
    .filter((n) => n > 0)
    .sort((a, b) => a - b)
  if (nonZero.length === 0) return [1, 2]
  const p33 = nonZero[Math.floor(nonZero.length * 0.33)] ?? 1
  const p67 = nonZero[Math.floor(nonZero.length * 0.67)] ?? p33 + 1
  return [p33, p67]
}

export type DensityLevel = 'none' | 'low' | 'mid' | 'high'

export function densityLevel(total: number, thresholds: [number, number]): DensityLevel {
  if (total === 0) return 'none'
  if (total < thresholds[0]) return 'low'
  if (total < thresholds[1]) return 'mid'
  return 'high'
}

// Format a RFC3339 timestamp as a local time string, e.g. "14:02".
export function formatLocalTime(ts: string, tz: string): string {
  const inst = Temporal.Instant.from(ts)
  const zdt = inst.toZonedDateTimeISO(tz)
  const h = String(zdt.hour).padStart(2, '0')
  const m = String(zdt.minute).padStart(2, '0')
  return `${h}:${m}`
}

// Format a RFC3339 timestamp as a local date string, e.g. "Wed May 27".
export function formatLocalDate(ts: string, tz: string): string {
  const inst = Temporal.Instant.from(ts)
  const zdt = inst.toZonedDateTimeISO(tz)
  const pd = zdt.toPlainDate()
  const dow = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'][pd.dayOfWeek - 1]
  const month = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
  ][pd.month - 1]
  return `${dow} ${month} ${pd.day}`
}

// Format a PlainDate as a local date subheader, e.g. "Wed, May 27, 2026".
export function formatDateHeader(d: Temporal.PlainDate): string {
  const dow = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'][d.dayOfWeek - 1]
  const month = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
  ][d.month - 1]
  return `${dow}, ${month} ${d.day}, ${d.year}`
}

// Extract the local PlainDate from a RFC3339 timestamp.
export function clipLocalDate(ts: string, tz: string): Temporal.PlainDate {
  return Temporal.Instant.from(ts).toZonedDateTimeISO(tz).toPlainDate()
}

// Returns true if d is within [from, to] (inclusive).
export function inRange(
  d: Temporal.PlainDate,
  from: Temporal.PlainDate,
  to: Temporal.PlainDate,
): boolean {
  return Temporal.PlainDate.compare(d, from) >= 0 && Temporal.PlainDate.compare(d, to) <= 0
}

// Returns true if d is more than MAX_RANGE_DAYS away from anchor.
export function outOfRangeCap(d: Temporal.PlainDate, anchor: Temporal.PlainDate): boolean {
  const diff = Math.abs(d.since(anchor).days)
  return diff > MAX_RANGE_DAYS
}

// Format duration seconds as "mm:ss".
export function formatDuration(s: number): string {
  const m = Math.floor(s / 60)
  const sec = Math.floor(s % 60)
  return `${m}:${String(sec).padStart(2, '0')}`
}
