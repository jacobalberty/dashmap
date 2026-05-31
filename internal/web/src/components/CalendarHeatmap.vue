<script setup lang="ts">
import { computed } from 'vue'
import { Temporal } from 'temporal-polyfill'
import { useBrowseStore } from '@/stores/browse'
import {
  buildMonthGrid,
  densityThresholds,
  densityLevel,
  inRange,
  outOfRangeCap,
  MAX_RANGE_DAYS,
} from '@/lib/datetime'

const store = useBrowseStore()

const grid = computed(() => buildMonthGrid(store.calendarMonth))
const thresholds = computed(() => densityThresholds(store.calendar))
const dayMap = computed(() => {
  const m = new Map<string, (typeof store.calendar)[0]>()
  for (const d of store.calendar) m.set(d.date, d)
  return m
})

const monthLabel = computed(() => {
  const ym = store.calendarMonth
  const months = [
    'January','February','March','April','May','June',
    'July','August','September','October','November','December',
  ]
  return `${months[ym.month - 1]} ${ym.year}`
})

// First-click pending state (before the range is finalised).
const pending = computed(() => {
  if (store.rangeFrom && !store.rangeTo) return store.rangeFrom
  return null
})

function isRangeStart(d: Temporal.PlainDate): boolean {
  return !!store.rangeFrom && Temporal.PlainDate.compare(d, store.rangeFrom) === 0
}

function isRangeEnd(d: Temporal.PlainDate): boolean {
  return !!store.rangeTo && Temporal.PlainDate.compare(d, store.rangeTo) === 0
}

function isInRange(d: Temporal.PlainDate): boolean {
  if (!store.rangeFrom || !store.rangeTo) return false
  return inRange(d, store.rangeFrom, store.rangeTo)
}

function isDisabled(d: Temporal.PlainDate): boolean {
  if (!pending.value) return false
  return outOfRangeCap(d, pending.value)
}

function cellClass(d: Temporal.PlainDate | null): string[] {
  if (!d) return ['cell', 'empty']
  const classes = ['cell']
  const iso = d.toString()
  const data = dayMap.value.get(iso)
  const level = data ? densityLevel(data.total, thresholds.value) : 'none'
  classes.push(`density-${level}`)
  if (isInRange(d)) classes.push('in-range')
  if (isRangeStart(d)) classes.push('range-start')
  if (isRangeEnd(d)) classes.push('range-end')
  if (isDisabled(d)) classes.push('disabled')
  if (
    store.rangeFrom &&
    !store.rangeTo &&
    Temporal.PlainDate.compare(d, store.rangeFrom) === 0
  )
    classes.push('first-click')
  return classes
}

function tooltip(d: Temporal.PlainDate | null): string {
  if (!d) return ''
  const data = dayMap.value.get(d.toString())
  if (!data || data.total === 0) return d.toString()
  return `${d} · ${data.total} clips (${data.recent} recent, ${data.saved} saved, ${data.sentry} sentry)`
}

function onCellClick(d: Temporal.PlainDate | null) {
  if (!d || isDisabled(d)) return

  if (!store.rangeFrom || (store.rangeFrom && store.rangeTo)) {
    // Start a new selection.
    store.rangeFrom = d
    store.rangeTo = null
    return
  }

  // Second click — complete the range.
  let from = store.rangeFrom
  let to = d
  if (Temporal.PlainDate.compare(to, from) < 0) {
    ;[from, to] = [to, from]
  }
  store.applyRange(from, to)
}

function prevMonth() {
  store.calendarMonth = store.calendarMonth.subtract({ months: 1 })
  store.loadCalendar()
}

function nextMonth() {
  store.calendarMonth = store.calendarMonth.add({ months: 1 })
  store.loadCalendar()
}
</script>

<template>
  <div class="calendar">
    <div class="cal-header">
      <button class="nav-btn" @click="prevMonth" aria-label="Previous month">‹</button>
      <span class="month-label">{{ monthLabel }}</span>
      <button class="nav-btn" @click="nextMonth" aria-label="Next month">›</button>
    </div>

    <div v-if="store.calendarLoading" class="cal-loading">…</div>

    <div v-else class="cal-grid">
      <div v-for="d in ['Mo','Tu','We','Th','Fr','Sa','Su']" :key="d" class="dow-header">{{ d }}</div>
      <div
        v-for="(cell, i) in grid"
        :key="i"
        :class="cellClass(cell)"
        :title="tooltip(cell)"
        :tabindex="cell ? 0 : -1"
        @click="onCellClick(cell)"
        @keydown.enter.prevent="onCellClick(cell)"
        @keydown.space.prevent="onCellClick(cell)"
      >
        {{ cell?.day ?? '' }}
      </div>
    </div>

    <div class="cal-legend">
      <span class="leg-item"><span class="leg-swatch density-low" />low</span>
      <span class="leg-item"><span class="leg-swatch density-mid" />mid</span>
      <span class="leg-item"><span class="leg-swatch density-high" />high</span>
      <span v-if="!store.rangeFrom" class="leg-hint">click a start day</span>
      <span v-else-if="!store.rangeTo" class="leg-hint">click an end day (max {{ MAX_RANGE_DAYS }}d)</span>
    </div>
  </div>
</template>

<style scoped>
.calendar {
  padding: 12px;
  border-bottom: 1px solid var(--border);
}

.cal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.month-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-1);
}

.nav-btn {
  background: none;
  border: none;
  color: var(--text-2);
  font-size: 18px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
  line-height: 1;
}
.nav-btn:hover { background: var(--bg-hover); color: var(--text-1); }

.cal-loading { text-align: center; color: var(--text-3); padding: 20px; }

.cal-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 2px;
}

.dow-header {
  text-align: center;
  font-size: 10px;
  font-weight: 600;
  color: var(--text-3);
  text-transform: uppercase;
  padding: 4px 0;
}

.cell {
  text-align: center;
  padding: 5px 2px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  color: var(--text-2);
  transition: background 0.1s;
  outline: none;
  min-width: 0;
  user-select: none;
}

.cell.empty { cursor: default; }
.cell.disabled { opacity: 0.25; cursor: not-allowed; }
.cell:not(.empty):not(.disabled):hover { background: var(--bg-hover); color: var(--text-1); }

/* Density shading */
.density-none { color: var(--text-3); }
.density-low  { color: var(--text-2); background: rgba(79,142,247,0.08); }
.density-mid  { color: var(--text-1); background: rgba(79,142,247,0.18); }
.density-high { color: var(--text-1); background: rgba(79,142,247,0.32); font-weight: 600; }

/* Range highlight */
.in-range { background: var(--accent-muted); color: var(--text-1); }
.range-start, .range-end {
  background: var(--accent) !important;
  color: #fff !important;
  font-weight: 600;
}
.first-click {
  background: var(--accent) !important;
  color: #fff !important;
  font-weight: 600;
}

.cal-legend {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
  font-size: 10px;
  color: var(--text-3);
}

.leg-item { display: flex; align-items: center; gap: 4px; }

.leg-swatch {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 2px;
}

.leg-hint { margin-left: auto; font-style: italic; }
</style>
