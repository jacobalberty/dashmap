# Followup: Timeline Scrubber

**Depends on:** Story 05  **Status:** Not started

**Goal:** Add a time-of-day scrubber to the browse screen so users can navigate within a
day at sub-day granularity — complementing the calendar (which picks days) with a moment
picker.

## What it would do

The scrubber sits below the map or at the bottom of the left panel. It shows a horizontal
time axis for the selected day (e.g. 08:00–22:00) with each clip rendered as a tick at its
`clip_ts`, colored by kind (sentry=amber, saved=blue, recent=slate). A draggable playhead
shows the current position; dragging or clicking selects the nearest clip and syncs the map
marker (◎) and list row.

## Two depth levels

1. **Minimal (clip selector)**: tick strip + draggable handle → selects the nearest clip by
   time, syncs map + list. Implemented almost entirely as a styled `<input type=range>` over the
   day's already-loaded `store.clips` filtered to the selected day. Low effort.

2. **Track-synced (position animation)**: the playhead also drives a "car position" dot that
   glides along the GPS track on the map, interpolated from `/api/track` point timestamps.
   Lets you replay the day's route over time without video. Notably more work — needs
   timestamp-to-position interpolation on the MapLibre `trackRuns`.

## Notes

- Build the minimal version first; the track-synced animation can be layered on top.
- The wireframe's `☐ autoplay next` checkbox belongs in Story 06 (playback), **not** here.
- Single-day ranges are the natural scope; multi-day scrubber UX is less clear — skip for now.
- No new backend calls needed: clips + track are already loaded by `store.applyRange`.
