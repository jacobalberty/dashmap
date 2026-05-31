# Followup: GPX / GeoJSON Track Export

**Depends on:** Story 05  **Status:** Not started

**Goal:** Add an Export button to the browse screen that downloads the selected range's GPS
track as GPX or GeoJSON — useful for importing into navigation apps, analysing the route
elsewhere, or archiving drives.

## What it would do

A small Export button in the browse header (or next to the track toggle on the map). Clicking
it builds the export file client-side from `store.trackRuns` (already loaded by `applyRange`)
and triggers a Blob download. Filename derived from the range: e.g.
`dashmap-2026-05-24_2026-05-29.gpx`.

No backend changes needed.

## Formats

### GeoJSON (`MultiLineString`)
```json
{
  "type": "Feature",
  "geometry": {
    "type": "MultiLineString",
    "coordinates": [
      [[lng, lat], [lng, lat], ...],  // run 0
      [[lng, lat], [lng, lat], ...]   // run 1
    ]
  },
  "properties": { "from": "2026-05-24", "to": "2026-05-29" }
}
```

### GPX (`<trk>` with one `<trkseg>` per run)
```xml
<?xml version="1.0"?>
<gpx version="1.1" creator="dashmap">
  <trk>
    <name>dashmap 2026-05-24 – 2026-05-29</name>
    <trkseg>
      <trkpt lat="37.77" lon="-122.41"><time>2026-05-24T09:11:00Z</time></trkpt>
      ...
    </trkseg>
    <trkseg>...</trkseg>
  </trk>
</gpx>
```

## Implementation sketch

```typescript
function exportGPX(runs: TrackPoint[][], range: { from: string; to: string }): void {
  const segs = runs.map((run) =>
    `<trkseg>${run.map((p) => `<trkpt lat="${p.lat}" lon="${p.lon}"><time>${p.ts}</time></trkpt>`).join('')}</trkseg>`
  ).join('')
  const xml = `<?xml version="1.0"?><gpx version="1.1" creator="dashmap"><trk><name>dashmap ${range.from} – ${range.to}</name>${segs}</trk></gpx>`
  downloadBlob(xml, `dashmap-${range.from}_${range.to}.gpx`, 'application/gpx+xml')
}

function downloadBlob(content: string, filename: string, type: string) {
  const url = URL.createObjectURL(new Blob([content], { type }))
  const a = document.createElement('a')
  a.href = url; a.download = filename; a.click()
  URL.revokeObjectURL(url)
}
```

## Notes

- `TrackPoint.ts` provides the ISO8601 timestamp for `<time>` and GeoJSON property.
- Offer both formats (two buttons or a dropdown). GeoJSON is simpler to implement; GPX is
  more broadly compatible with cycling/hiking apps and car loggers.
- If `store.trackRuns` is empty (no GPS data in range), disable the button.
