import { ref } from 'vue'

const PREF_KEY = 'posta_chat_alerts'

/** Whether this browser plays a sound and shows notifications for chats; on unless the agent turned it off. */
export const alertsOn = ref(readPref())

function readPref(): boolean {
  try {
    return localStorage.getItem(PREF_KEY) !== 'off'
  } catch {
    return true
  }
}

export async function setAlerts(on: boolean) {
  alertsOn.value = on
  try {
    localStorage.setItem(PREF_KEY, on ? 'on' : 'off')
  } catch {
    // Storage unavailable: the choice lasts this session only.
  }
  // Browsers only let a click ask for permission, so the toggle is where it is asked.
  if (on && typeof Notification !== 'undefined' && Notification.permission === 'default') {
    await Notification.requestPermission().catch(() => 'denied')
  }
  if (on) unlockAudio()
}

// Synthesised with Web Audio: no audio file to ship, as in Posta Webmail.
type Tone = [freq: number, at: number, gain?: number]
const TUNES: Record<'chat' | 'message', Tone[]> = {
  chat: [[784, 0, 0.8], [1046.5, 0.14, 0.9], [1318.5, 0.28]],
  message: [[1318.5, 0], [1760, 0.11, 0.8]],
}

let audio: AudioContext | null = null

function context(): AudioContext | null {
  if (audio) return audio
  const AC = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
  if (!AC) return null
  audio = new AC()
  return audio
}

/** Browsers keep audio muted until the page was used, so the first click or key unlocks it. */
export function unlockAudio() {
  const c = context()
  if (c && c.state === 'suspended') void c.resume()
}

if (typeof window !== 'undefined') {
  const unlock = () => {
    unlockAudio()
    window.removeEventListener('pointerdown', unlock)
    window.removeEventListener('keydown', unlock)
  }
  window.addEventListener('pointerdown', unlock)
  window.addEventListener('keydown', unlock)
}

function play(kind: keyof typeof TUNES) {
  const c = context()
  if (!c || c.state !== 'running') return
  const master = c.createGain()
  master.gain.value = 0.18
  master.connect(c.destination)
  const start = c.currentTime + 0.02
  for (const [freq, at, gain = 1] of TUNES[kind]) {
    for (const [ratio, level] of [[1, 1], [4, 0.22]] as const) {
      const osc = c.createOscillator()
      const env = c.createGain()
      osc.type = 'sine'
      osc.frequency.value = freq * ratio
      // Upper partials die faster, which is what makes a struck sound rather than an organ tone.
      const decay = 0.45 / Math.sqrt(ratio)
      env.gain.setValueAtTime(0, start + at)
      env.gain.linearRampToValueAtTime(gain * level, start + at + 0.004)
      env.gain.exponentialRampToValueAtTime(0.0001, start + at + 0.004 + decay)
      osc.connect(env).connect(master)
      osc.start(start + at)
      osc.stop(start + at + decay + 0.05)
    }
  }
}

export interface ChatAlert {
  kind: 'chat' | 'message'
  title: string
  body: string
  /** One notification per conversation: a newer one replaces it instead of stacking. */
  tag: string
  onClick: () => void
}

/** Rings for a chat: always a sound, plus a system notification while the agent looks elsewhere. */
export function alertChat(a: ChatAlert) {
  if (!alertsOn.value) return
  play(a.kind)
  const away = document.hidden || !document.hasFocus()
  if (!away || typeof Notification === 'undefined' || Notification.permission !== 'granted') return
  try {
    const n = new Notification(a.title, { body: a.body, tag: a.tag, icon: '/favicon-96x96.png' })
    n.onclick = () => {
      window.focus()
      a.onClick()
      n.close()
    }
  } catch {
    // Some browsers only allow notifications from a service worker; the sound still rang.
  }
}

let baseTitle = typeof document !== 'undefined' ? document.title : ''
let badge = 0

function render() {
  document.title = badge > 0 ? `(${badge}) ${baseTitle}` : baseTitle
}

/** The router hands every page title here, so the unread count survives navigation. */
export function setPageTitle(title: string) {
  baseTitle = title
  render()
}

/** Chats waiting for an answer, shown in the tab title. */
export function setTitleBadge(count: number) {
  badge = count
  render()
}
