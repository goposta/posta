import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { conversationsApi } from '../api/chat'
import type {
  ChatAgentPresence,
  ChatConversationEvent,
  ChatMessageEvent,
  ChatPresenceStatus,
  ChatReadEvent,
  ChatTypingEvent,
  ChatVisitorEvent,
} from '../api/types'
import { apiMessage } from '../composables/apiError'
import { useAuthStore } from './auth'
import { useNotificationStore } from './notification'
import { useWorkspaceStore } from './workspace'

/** What the stream hands to subscribers. `reconnected` means events may have been missed: refetch. */
export type ChatStreamEvent =
  | { type: 'chat.message'; data: ChatMessageEvent }
  | { type: 'chat.conversation'; data: ChatConversationEvent }
  | { type: 'chat.typing'; data: ChatTypingEvent }
  | { type: 'chat.read'; data: ChatReadEvent }
  | { type: 'chat.visitor'; data: ChatVisitorEvent }
  | { type: 'reconnected' }

type Listener = (event: ChatStreamEvent) => void

const MIN_BACKOFF = 1000
const MAX_BACKOFF = 30000
// The server forgets a heartbeat after 75 s; one every 30 s survives a miss.
const HEARTBEAT_MS = 30000
const PRESENCE_KEY = 'posta_chat_presence_'

function savedStatus(workspaceId: number): ChatPresenceStatus {
  try {
    const v = localStorage.getItem(PRESENCE_KEY + workspaceId)
    return v === 'online' || v === 'away' ? v : 'offline'
  } catch {
    return 'offline'
  }
}

function saveStatus(workspaceId: number, status: ChatPresenceStatus) {
  try {
    localStorage.setItem(PRESENCE_KEY + workspaceId, status)
  } catch {
    // Storage unavailable: the choice lasts this session only.
  }
}

// The dashboard's single chat event stream: views subscribe here instead of opening their own.
export const useChatStore = defineStore('chat', () => {
  /** null until probed; false when the server runs without POSTA_CHAT_ENABLED. */
  const enabled = ref<boolean | null>(null)
  const awaitingReply = ref(0)
  const connected = ref(false)
  /** This agent's own availability; agents opt in, so it starts offline. */
  const myStatus = ref<ChatPresenceStatus>('offline')
  /** Agents online or away, by user id. */
  const agents = ref<Record<number, ChatPresenceStatus>>({})
  const onlineCount = computed(() => Object.values(agents.value).filter((s) => s === 'online').length)

  const listeners = new Set<Listener>()
  let source: EventSource | null = null
  let backoff = MIN_BACKOFF
  let retryTimer: ReturnType<typeof setTimeout> | null = null
  let summaryTimer: ReturnType<typeof setTimeout> | null = null
  let everOpened = false
  let generation = 0
  let heartbeatTimer: ReturnType<typeof setInterval> | null = null
  /** The workspace the heartbeat speaks for, kept for the goodbye. */
  let presenceWs: number | null = null

  function canAnswer(): boolean {
    return useWorkspaceStore().canEdit
  }

  async function loadPresence() {
    try {
      const res = await conversationsApi.presence()
      const next: Record<number, ChatPresenceStatus> = {}
      for (const a of res.data.data as ChatAgentPresence[]) next[a.user_id] = a.status
      agents.value = next
    } catch {
      agents.value = {}
    }
  }

  function heartbeat() {
    if (myStatus.value === 'offline') return
    conversationsApi.setPresence(myStatus.value).catch((e) => {
      // 403: the plan's agents are all online already. Say so and stand down.
      if (e?.response?.status !== 403) return
      useNotificationStore().error(apiMessage(e, 'Your plan allows no more chat agents online'))
      const ws = presenceWs
      stopHeartbeat()
      myStatus.value = 'offline'
      setOwn('offline')
      if (ws !== null) saveStatus(ws, 'offline')
    })
    setOwn(myStatus.value)
  }

  // Keep this agent's own entry current; the stream may not echo it back.
  function setOwn(status: ChatPresenceStatus) {
    const me = useAuthStore().user?.id
    if (me === undefined) return
    const next = { ...agents.value }
    if (status === 'offline') delete next[me]
    else next[me] = status
    agents.value = next
  }

  function startHeartbeat() {
    stopHeartbeat()
    if (myStatus.value === 'offline' || !canAnswer()) return
    heartbeat()
    heartbeatTimer = setInterval(heartbeat, HEARTBEAT_MS)
  }

  function stopHeartbeat() {
    if (heartbeatTimer) clearInterval(heartbeatTimer)
    heartbeatTimer = null
  }

  /** Change this agent's availability, remembered per workspace. */
  async function setStatus(status: ChatPresenceStatus) {
    const ws = presenceWs
    if (ws === null || !canAnswer()) return
    myStatus.value = status
    saveStatus(ws, status)
    if (status === 'offline') {
      stopHeartbeat()
      setOwn('offline')
      await conversationsApi.setPresence('offline').catch(() => {})
      return
    }
    startHeartbeat()
  }

  /** Say goodbye without forgetting the choice: the next start resumes it. */
  function leave() {
    stopHeartbeat()
    if (presenceWs !== null && myStatus.value !== 'offline') conversationsApi.goOffline(presenceWs)
  }

  // A hidden or closed tab goes offline at once instead of 75 s later; coming back resumes the heartbeat.
  if (typeof window !== 'undefined') {
    window.addEventListener('pagehide', () => leave())
    window.addEventListener('pageshow', (e) => {
      if ((e as PageTransitionEvent).persisted && presenceWs !== null) startHeartbeat()
    })
  }

  function subscribe(fn: Listener): () => void {
    listeners.add(fn)
    return () => listeners.delete(fn)
  }

  function emit(event: ChatStreamEvent) {
    for (const fn of listeners) {
      try {
        fn(event)
      } catch {
        // One broken view must not stop the others hearing the event.
      }
    }
  }

  async function refreshSummary(): Promise<boolean> {
    try {
      const res = await conversationsApi.summary()
      awaitingReply.value = res.data.data.awaiting_reply
      enabled.value = true
      return true
    } catch (e: any) {
      if (e?.response?.status === 404) enabled.value = false
      return false
    }
  }

  // Bursts of events (a visitor typing several lines) refresh the count once.
  function scheduleSummary() {
    if (summaryTimer) clearTimeout(summaryTimer)
    summaryTimer = setTimeout(() => {
      summaryTimer = null
      refreshSummary()
    }, 500)
  }

  function parse<T>(e: MessageEvent): T | null {
    try {
      return JSON.parse(e.data) as T
    } catch {
      return null
    }
  }

  function connect(gen: number) {
    if (gen !== generation || typeof EventSource === 'undefined') return
    const es = new EventSource(conversationsApi.streamUrl(), { withCredentials: true })
    source = es
    es.onopen = () => {
      connected.value = true
      backoff = MIN_BACKOFF
      if (everOpened) {
        emit({ type: 'reconnected' })
        scheduleSummary()
      }
      everOpened = true
    }
    es.addEventListener('chat.message', (e) => {
      const data = parse<ChatMessageEvent>(e as MessageEvent)
      if (!data) return
      emit({ type: 'chat.message', data })
      scheduleSummary()
    })
    es.addEventListener('chat.conversation', (e) => {
      const data = parse<ChatConversationEvent>(e as MessageEvent)
      if (!data) return
      if (data.started) {
        useNotificationStore().info(`New chat from ${data.visitor_name || 'a visitor'}`)
      }
      emit({ type: 'chat.conversation', data })
      scheduleSummary()
    })
    es.addEventListener('chat.presence', (e) => {
      const data = parse<ChatAgentPresence>(e as MessageEvent)
      if (!data) return
      const next = { ...agents.value }
      if (data.status === 'offline') delete next[data.user_id]
      else next[data.user_id] = data.status
      agents.value = next
    })
    for (const type of ['chat.typing', 'chat.read', 'chat.visitor'] as const) {
      es.addEventListener(type, (e) => {
        const data = parse<any>(e as MessageEvent)
        if (data) emit({ type, data } as ChatStreamEvent)
      })
    }
    es.onerror = () => {
      connected.value = false
      // The browser retries a dropped connection itself but gives up on an HTTP error, so we take over then, with backoff.
      if (es.readyState !== EventSource.CLOSED) return
      es.close()
      if (source === es) source = null
      if (retryTimer) clearTimeout(retryTimer)
      retryTimer = setTimeout(() => connect(gen), backoff)
      backoff = Math.min(backoff * 2, MAX_BACKOFF)
    }
  }

  /** (Re)start for the current workspace. Safe to call repeatedly. */
  async function start() {
    // Restarting for the same workspace must not flap this agent offline, which would take the widget offline for visitors too.
    const ws = useWorkspaceStore().currentWorkspaceId
    stop(presenceWs !== null && presenceWs !== (ws ?? null))
    const gen = ++generation
    if (!(await refreshSummary()) || gen !== generation) return
    presenceWs = ws ?? null
    myStatus.value = ws ? savedStatus(ws) : 'offline'
    await loadPresence()
    if (gen !== generation) return
    // Another tab may already have this agent online: keep that rather than a stale local choice.
    const me = useAuthStore().user?.id
    const live = me !== undefined ? agents.value[me] : undefined
    if (ws && live && live !== myStatus.value) {
      myStatus.value = live
      saveStatus(ws, live)
    }
    startHeartbeat()
    connect(gen)
  }

  function stop(goOffline = true) {
    if (goOffline) leave()
    else stopHeartbeat()
    presenceWs = null
    agents.value = {}
    generation++
    if (retryTimer) clearTimeout(retryTimer)
    if (summaryTimer) clearTimeout(summaryTimer)
    retryTimer = summaryTimer = null
    source?.close()
    source = null
    connected.value = false
    everOpened = false
    backoff = MIN_BACKOFF
  }

  return {
    enabled, awaitingReply, connected, myStatus, agents, onlineCount,
    subscribe, start, stop, refreshSummary, setStatus,
  }
})
