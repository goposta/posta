// IANA timezone names for timezone pickers, with their current UTC offset.
// The list comes from the browser (Intl.supportedValuesOf) so it tracks the
// tz database instead of a hand-maintained subset; the backend validates the
// chosen name against Go's tz database (okapi `format:"timezone"`).

export interface TimezoneOption {
  value: string
  label: string
}

// Used only when the browser cannot enumerate zones (pre-2022 engines).
const FALLBACK_ZONES = [
  'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles',
  'America/Sao_Paulo', 'Europe/London', 'Europe/Paris', 'Europe/Berlin', 'Europe/Moscow',
  'Asia/Tokyo', 'Asia/Shanghai', 'Asia/Kolkata', 'Asia/Dubai', 'Asia/Singapore',
  'Australia/Sydney', 'Pacific/Auckland', 'Africa/Kinshasa', 'Africa/Lubumbashi',
  'Africa/Nairobi', 'Africa/Lagos', 'Africa/Johannesburg', 'Africa/Cairo',
]

function zoneNames(): string[] {
  try {
    const names = Intl.supportedValuesOf('timeZone')
    if (names.length > 0) return names
  } catch {
    /* fall through */
  }
  return FALLBACK_ZONES
}

function utcOffset(zone: string, at: Date): string {
  try {
    const part = new Intl.DateTimeFormat('en-US', { timeZone: zone, timeZoneName: 'longOffset' })
      .formatToParts(at)
      .find((p) => p.type === 'timeZoneName')?.value
    // "GMT" alone means +00:00; otherwise it is "GMT+05:30".
    return !part || part === 'GMT' ? 'UTC+00:00' : part.replace('GMT', 'UTC')
  } catch {
    return ''
  }
}

let cached: TimezoneOption[] | null = null

// timezoneOptions returns UTC first, then every zone sorted by name. A saved
// value missing from the browser's list (an alias such as Asia/Calcutta, or a
// zone this engine does not know) is kept so opening the form never changes it.
export function timezoneOptions(current?: string): TimezoneOption[] {
  if (!cached) {
    const now = new Date()
    const names = zoneNames().filter((z) => z !== 'UTC').sort()
    cached = [{ value: 'UTC', label: 'UTC' }].concat(
      names.map((z) => {
        const offset = utcOffset(z, now)
        return { value: z, label: offset ? `${z} (${offset})` : z }
      }),
    )
  }
  if (current && !cached.some((o) => o.value === current)) {
    return [{ value: current, label: current }, ...cached]
  }
  return cached
}
