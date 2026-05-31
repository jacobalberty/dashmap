import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import { Temporal } from 'temporal-polyfill'
import type { Car, CalendarDay, ClipListItem, ClipMarker, TrackPoint, TrackMeta } from '@/types'
import {
  getCars,
  getCalendar,
  getClips,
  getClipMarkers,
  getTrack,
  getStatus,
} from '@/lib/api'
import {
  localTz,
  currentMonth,
  rangeToUTC,
  monthBoundsUTC,
} from '@/lib/datetime'

export const useBrowseStore = defineStore('browse', () => {
  // ── car ────────────────────────────────────────────────────────────────────
  const cars = ref<Car[]>([])
  const selectedCarKey = ref<string>('')
  const isLive = ref(false)

  // ── calendar / month ────────────────────────────────────────────────────────
  const calendarMonth = ref<Temporal.PlainYearMonth>(currentMonth())
  const calendar = ref<CalendarDay[]>([])
  const calendarLoading = ref(false)

  // ── active range ────────────────────────────────────────────────────────────
  const rangeFrom = ref<Temporal.PlainDate | null>(null)
  const rangeTo = ref<Temporal.PlainDate | null>(null)

  // ── map data ────────────────────────────────────────────────────────────────
  const markers = ref<ClipMarker[]>([])
  const markersCapped = ref(false)
  const trackRuns = ref<TrackPoint[][]>([])
  const trackMeta = ref<TrackMeta | null>(null)
  const mapLoading = ref(false)

  // ── clip list ────────────────────────────────────────────────────────────────
  const clips = ref<ClipListItem[]>([])
  const clipsOffset = ref(0)
  const clipsHasMore = ref(false)
  const clipsLoading = ref(false)

  // ── selection ────────────────────────────────────────────────────────────────
  const selectedClipId = ref<number | null>(null)

  // ── error ────────────────────────────────────────────────────────────────────
  const error = ref<string | null>(null)

  // ── computed ─────────────────────────────────────────────────────────────────
  const tz = computed(() => localTz())
  const hasRange = computed(() => rangeFrom.value !== null && rangeTo.value !== null)
  const rangeUTC = computed(() => {
    if (!rangeFrom.value || !rangeTo.value) return null
    return rangeToUTC(rangeFrom.value, rangeTo.value, tz.value)
  })

  // ── actions ───────────────────────────────────────────────────────────────────

  async function loadCars() {
    try {
      cars.value = await getCars()
      if (cars.value.length > 0 && !selectedCarKey.value) {
        selectedCarKey.value = cars.value[0].key
      }
    } catch (e) {
      error.value = String(e)
    }
  }

  async function loadCalendar(month?: Temporal.PlainYearMonth) {
    const ym = month ?? calendarMonth.value
    calendarMonth.value = ym
    calendarLoading.value = true
    error.value = null
    try {
      const { from, to } = monthBoundsUTC(ym, tz.value)
      calendar.value = await getCalendar({
        from,
        to,
        tz: tz.value,
        car: selectedCarKey.value || undefined,
      })
    } catch (e) {
      error.value = String(e)
    } finally {
      calendarLoading.value = false
    }
  }

  async function applyRange(from: Temporal.PlainDate, to: Temporal.PlainDate) {
    rangeFrom.value = from
    rangeTo.value = to
    selectedClipId.value = null
    clips.value = []
    clipsOffset.value = 0
    clipsHasMore.value = false
    error.value = null
    mapLoading.value = true
    clipsLoading.value = true

    const utc = rangeToUTC(from, to, tz.value)
    const car = selectedCarKey.value || undefined

    try {
      const [markersRes, trackRes, clipsRes] = await Promise.all([
        getClipMarkers({ from: utc.from, to: utc.to, car }),
        getTrack({ from: utc.from, to: utc.to, car }),
        getClips({ from: utc.from, to: utc.to, car, limit: 100 }),
      ])

      markers.value = markersRes.data
      markersCapped.value = markersRes.meta.capped

      trackRuns.value = trackRes.data
      trackMeta.value = trackRes.meta

      clips.value = clipsRes.data
      clipsOffset.value = clipsRes.data.length
      clipsHasMore.value = clipsRes.meta.has_more
    } catch (e) {
      error.value = String(e)
    } finally {
      mapLoading.value = false
      clipsLoading.value = false
    }
  }

  async function loadMoreClips() {
    if (!hasRange.value || !clipsHasMore.value || clipsLoading.value) return
    clipsLoading.value = true
    const utc = rangeUTC.value!
    const car = selectedCarKey.value || undefined
    try {
      const res = await getClips({
        from: utc.from,
        to: utc.to,
        car,
        limit: 100,
        offset: clipsOffset.value,
      })
      clips.value.push(...res.data)
      clipsOffset.value += res.data.length
      clipsHasMore.value = res.meta.has_more
    } catch (e) {
      error.value = String(e)
    } finally {
      clipsLoading.value = false
    }
  }

  function selectClip(id: number | null) {
    selectedClipId.value = id
  }

  async function setCar(key: string) {
    if (key === selectedCarKey.value) return
    selectedCarKey.value = key
    selectedClipId.value = null
    // Reload calendar and range data for the new car.
    await loadCalendar()
    if (hasRange.value) {
      await applyRange(rangeFrom.value!, rangeTo.value!)
    }
  }

  async function loadStatus() {
    try {
      const st = await getStatus(selectedCarKey.value || undefined)
      isLive.value = st.car.live
    } catch {
      isLive.value = false
    }
  }

  return {
    // state
    cars,
    selectedCarKey,
    isLive,
    calendarMonth,
    calendar,
    calendarLoading,
    rangeFrom,
    rangeTo,
    markers,
    markersCapped,
    trackRuns,
    trackMeta,
    mapLoading,
    clips,
    clipsOffset,
    clipsHasMore,
    clipsLoading,
    selectedClipId,
    error,
    // computed
    tz,
    hasRange,
    rangeUTC,
    // actions
    loadCars,
    loadCalendar,
    applyRange,
    loadMoreClips,
    selectClip,
    setCar,
    loadStatus,
  }
})
