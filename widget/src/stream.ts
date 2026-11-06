import { Api, Conversation, Message } from './api'
import { SSEParser, backoff } from './sse'

export interface StreamHandlers {
  message(m: Message): void
  conversation(c: Conversation): void
  // typing says an agent is writing; read says how far the agents have read.
  typing(name?: string): void
  read(seq: number): void
  // gone is called when the conversation can no longer be read: the token
  // was revoked or the conversation erased. The stream stops.
  gone(): void
}

const POLL_MS = 10000
const HIDDEN_CLOSE_MS = 5 * 60 * 1000

// ConversationStream keeps a visitor up to date with one conversation. It
// reads server-sent events through fetch, resumes from the last sequence
// number it saw, falls back to polling when the server is at its stream
// limit, and lets go of the connection while the tab stays hidden.
export class ConversationStream {
  private lastSeq: number
  private ctrl?: AbortController
  private timer?: ReturnType<typeof setTimeout>
  private hiddenTimer?: ReturnType<typeof setTimeout>
  private attempt = 0
  private stopped = false
  private paused = false

  constructor(
    private api: Api,
    private conv: string,
    since: number,
    private on: StreamHandlers,
  ) {
    this.lastSeq = since
    document.addEventListener('visibilitychange', this.onVisibility)
    this.connect()
  }

  stop(): void {
    this.stopped = true
    this.ctrl?.abort()
    clearTimeout(this.timer)
    clearTimeout(this.hiddenTimer)
    document.removeEventListener('visibilitychange', this.onVisibility)
  }

  // seen records a sequence number the widget learned about another way,
  // such as the response to its own send.
  seen(seq: number): void {
    if (seq > this.lastSeq) this.lastSeq = seq
  }

  private onVisibility = () => {
    if (document.visibilityState === 'hidden') {
      this.hiddenTimer = setTimeout(() => {
        this.paused = true
        this.ctrl?.abort()
        clearTimeout(this.timer)
      }, HIDDEN_CLOSE_MS)
      return
    }
    clearTimeout(this.hiddenTimer)
    if (this.paused) {
      this.paused = false
      this.attempt = 0
      this.connect()
    }
  }

  private schedule(fn: () => void, ms: number) {
    clearTimeout(this.timer)
    if (!this.stopped && !this.paused) this.timer = setTimeout(fn, ms)
  }

  private deliver(m: Message) {
    if (m.seq > this.lastSeq) this.lastSeq = m.seq
    this.on.message(m)
  }

  private async connect() {
    if (this.stopped || this.paused) return
    this.ctrl = new AbortController()
    const headers = this.api.headers()
    headers.Accept = 'text/event-stream'
    headers['Last-Event-ID'] = String(this.lastSeq)
    let res: Response
    try {
      res = await fetch(this.api.url(`/conversations/${encodeURIComponent(this.conv)}/stream`), {
        headers,
        signal: this.ctrl.signal,
        credentials: 'omit',
        mode: 'cors',
        cache: 'no-store',
      })
    } catch {
      return this.retry()
    }
    if (res.status === 401 || res.status === 404) {
      this.stop()
      this.on.gone()
      return
    }
    if (res.status === 503) {
      // The server holds too many streams: poll until it has room.
      return this.poll()
    }
    if (!res.ok || !res.body) return this.retry()

    this.attempt = 0
    const reader = res.body.pipeThrough(new TextDecoderStream()).getReader()
    const parser = new SSEParser()
    try {
      for (;;) {
        const { value, done } = await reader.read()
        if (done) break
        for (const ev of parser.push(value)) this.handle(ev.event, ev.data)
      }
    } catch {
      // Aborted or dropped; fall through to reconnect.
    }
    this.retry()
  }

  private handle(event: string, data: string) {
    let payload: any
    try {
      payload = JSON.parse(data)
    } catch {
      return
    }
    if (event === 'chat.message' && typeof payload?.seq === 'number') this.deliver(payload)
    else if (event === 'chat.conversation' && payload?.id) this.on.conversation(payload)
    else if (event === 'chat.typing' && payload?.author === 'agent') this.on.typing(payload.name)
    else if (event === 'chat.read' && payload?.reader === 'agent' && typeof payload.seq === 'number') this.on.read(payload.seq)
  }

  private retry() {
    this.schedule(() => this.connect(), backoff(this.attempt++))
  }

  private async poll() {
    try {
      const items = await this.api.messages(this.conv, this.lastSeq)
      items.forEach((m) => this.deliver(m))
    } catch (e: any) {
      if (e?.status === 401 || e?.status === 404) {
        this.stop()
        this.on.gone()
        return
      }
    }
    // Try the stream again now and then; polling is the fallback, not the plan.
    this.schedule(() => (Math.random() < 0.2 ? this.connect() : this.poll()), POLL_MS)
  }
}
