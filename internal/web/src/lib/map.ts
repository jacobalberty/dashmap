import * as maplibregl from 'maplibre-gl'
import { Protocol } from 'pmtiles'
import { layers as basemapLayers, namedFlavor } from '@protomaps/basemaps'
import type { StyleSpecification } from 'maplibre-gl'

let protocolRegistered = false

export function initPMTilesProtocol() {
  if (protocolRegistered) return
  protocolRegistered = true
  const protocol = new Protocol({ metadata: true })
  maplibregl.addProtocol('pmtiles', protocol.tile)
}

const SPRITES = 'https://protomaps.github.io/basemaps-assets/sprites/v4/dark'
const GLYPHS =
  'https://protomaps.github.io/basemaps-assets/fonts/{fontstack}/{range}.pbf'
const SOURCE_NAME = 'protomaps'

export function createBasemapStyle(): StyleSpecification {
  return {
    version: 8,
    glyphs: GLYPHS,
    sprite: SPRITES,
    sources: {
      [SOURCE_NAME]: {
        type: 'vector',
        url: 'pmtiles:///tiles/basemap.pmtiles',
        attribution: '© <a href="https://openstreetmap.org/copyright">OpenStreetMap</a>',
      },
    },
    // @ts-expect-error basemapLayers returns compatible LayerSpecification[]
    layers: basemapLayers(SOURCE_NAME, namedFlavor('dark'), { lang: 'en' }),
  }
}

export function createBlankStyle(): StyleSpecification {
  return {
    version: 8,
    sources: {},
    layers: [
      {
        id: 'background',
        type: 'background',
        paint: { 'background-color': '#13141a' },
      },
    ],
  }
}

// Add a right-pointing arrow image to the map for track direction indicators.
export function addArrowImage(map: maplibregl.Map, color = '#4f8ef7') {
  const size = 20
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d')!
  ctx.clearRect(0, 0, size, size)
  ctx.fillStyle = color
  ctx.beginPath()
  ctx.moveTo(3, 4)
  ctx.lineTo(size - 3, size / 2)
  ctx.lineTo(3, size - 4)
  ctx.closePath()
  ctx.fill()
  const imageData = ctx.getImageData(0, 0, size, size)
  map.addImage('track-arrow', imageData, { pixelRatio: 2 })
}

// Add a circle image for start/end pins.
export function addPinImage(map: maplibregl.Map, name: string, color: string) {
  const size = 14
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d')!
  ctx.clearRect(0, 0, size, size)
  ctx.beginPath()
  ctx.arc(size / 2, size / 2, size / 2 - 1.5, 0, Math.PI * 2)
  ctx.fillStyle = color
  ctx.fill()
  ctx.strokeStyle = 'rgba(255,255,255,0.8)'
  ctx.lineWidth = 1.5
  ctx.stroke()
  map.addImage(name, ctx.getImageData(0, 0, size, size), { pixelRatio: 2 })
}

// Attempt to load the basemap style; fall back to blank if the pmtiles 404s.
export async function resolveMapStyle(): Promise<StyleSpecification> {
  try {
    const res = await fetch('/tiles/basemap.pmtiles', { method: 'HEAD' })
    if (res.ok) return createBasemapStyle()
  } catch {
    // network error or not configured
  }
  return createBlankStyle()
}
