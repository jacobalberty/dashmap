<script setup lang="ts">
import { computed } from 'vue'
import type { Telemetry } from '@/lib/playback'
import {
  formatGear,
  formatAutopilot,
  formatSpeedMph,
  formatSpeedKmh,
  headingCardinal,
} from '@/lib/playback'

const props = defineProps<{
  telemetry: Telemetry | null
  speedUnit: 'mph' | 'kmh'
}>()

const speed = computed(() => {
  if (!props.telemetry) return null
  return props.speedUnit === 'mph'
    ? `${formatSpeedMph(props.telemetry.speedMps)} mph`
    : `${formatSpeedKmh(props.telemetry.speedMps)} km/h`
})
</script>

<template>
  <div v-if="telemetry" class="sei-hud">
    <span class="hud-speed">{{ speed }}</span>
    <span class="hud-chip">{{ formatGear(telemetry.gear) }}</span>
    <span v-if="telemetry.blinkerLeft" class="hud-chip blink">◀</span>
    <span v-if="telemetry.blinkerRight" class="hud-chip blink">▶</span>
    <span v-if="telemetry.brakeApplied" class="hud-chip brake">BRK</span>
    <span class="hud-sep" />
    <span class="hud-item" title="Autopilot">{{ formatAutopilot(telemetry.autopilot) }}</span>
    <span class="hud-item" title="Steering">{{ telemetry.steeringWheelAngle.toFixed(1) }}°</span>
    <span class="hud-item" title="Heading">
      {{ Math.round(telemetry.headingDeg) }}° {{ headingCardinal(telemetry.headingDeg) }}
    </span>
    <span class="hud-sep" />
    <span class="hud-item coords">
      {{ telemetry.lat.toFixed(5) }}, {{ telemetry.lon.toFixed(5) }}
    </span>
  </div>
</template>

<style scoped>
.sei-hud {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 5px 14px;
  background: rgba(10,10,12,0.85);
  border-top: 1px solid var(--border, #2a2c35);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.hud-speed {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-1, #e2e4ea);
}

.hud-chip {
  background: rgba(255,255,255,0.08);
  border-radius: 3px;
  padding: 1px 5px;
  color: var(--text-2, #9199a6);
  font-weight: 600;
}

.hud-chip.blink { color: #fbbf24; }
.hud-chip.brake { color: #f87171; }

.hud-item {
  color: var(--text-2, #9199a6);
}

.hud-item.coords {
  font-family: monospace;
  font-size: 11px;
  color: var(--text-3, #565d6a);
}

.hud-sep {
  width: 1px;
  height: 14px;
  background: var(--border, #2a2c35);
}
</style>
