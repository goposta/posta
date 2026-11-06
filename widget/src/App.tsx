import { Fragment } from 'preact'
import { useEffect, useRef, useState } from 'preact/hooks'
import { Api, ApiError, Attachment, Conversation, Identity, WidgetConfig } from './api'
import { Strings } from './i18n'
import { linkify } from './linkify'
import { load, save } from './storage'
import { ConversationStream } from './stream'
import { Local, merge } from './thread'

export interface Controller {
  open(): void
  close(): void
  emit(event: 'message' | 'open' | 'close', payload?: unknown): void
}

interface Props {
  api: Api
  widgetKey: string
  config: WidgetConfig
  t: Strings
  bind(ctl: {
    open(): void
    close(): void
    identify(identity: Identity): void
    setAttributes(attributes: Record<string, unknown>): void
  }): void
  emit: Controller['emit']
}

const ACCEPT = 'image/png,image/jpeg,image/gif,image/webp,application/pdf'

function size(n: number): string {
  return n < 1024 * 1024 ? `${Math.max(1, Math.round(n / 1024))} KB` : `${(n / 1024 / 1024).toFixed(1)} MB`
}

// File shows one attachment. Images load through the visitor's token into
// an object URL; other files download on click.
function File_({ api, conv, message, idx, att }: { api: Api; conv?: string; message: string; idx: number; att: Attachment }) {
  const [url, setURL] = useState<string | null>(null)
  const image = att.content_type.startsWith('image/')
  useEffect(() => {
    if (!image || !conv || !message) return
    let objectURL: string | null = null
    api.attachment(conv, message, idx).then((blob) => {
      objectURL = URL.createObjectURL(blob)
      setURL(objectURL)
    }).catch(() => {})
    return () => {
      if (objectURL) URL.revokeObjectURL(objectURL)
    }
  }, [conv, message, idx])
  if (image && url) {
    return (
      <a href={url} target="_blank" rel="noopener noreferrer">
        <img class="att-img" src={url} alt={att.filename} />
      </a>
    )
  }
  const download = async () => {
    if (!conv || !message) return
    const blob = await api.attachment(conv, message, idx)
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = att.filename
    a.click()
    setTimeout(() => URL.revokeObjectURL(a.href), 1000)
  }
  return (
    <button type="button" class="att-file" onClick={download}>
      {att.filename} · {size(att.size)}
    </button>
  )
}

const NONCE_MAX_AGE_MS = 25 * 60 * 1000
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

function uuid(): string {
  if (crypto.randomUUID) return crypto.randomUUID()
  const b = crypto.getRandomValues(new Uint8Array(16))
  return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')
}

function Body({ text }: { text: string }) {
  return (
    <>
      {linkify(text).map((p, i) =>
        p.href ? (
          <a key={i} href={p.href} target="_blank" rel="noopener noreferrer nofollow">
            {p.text}
          </a>
        ) : (
          p.text
        ),
      )}
    </>
  )
}

export function App({ api, widgetKey, config, t, bind, emit }: Props) {
  const saved = useRef(load(widgetKey))
  const [open, setOpen] = useState(false)
  const [messages, setMessages] = useState<Local[]>([])
  const [conv, setConv] = useState<Conversation | null>(null)
  const [unread, setUnread] = useState(0)
  const [text, setText] = useState('')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [agentTyping, setAgentTyping] = useState<string | null>(null)
  const [agentRead, setAgentRead] = useState(0)
  const typingTimer = useRef<ReturnType<typeof setTimeout>>()
  const lastTypingSent = useRef(0)
  const lastReadSent = useRef(0)
  const identity = useRef<Identity | undefined>(undefined)
  const attributes = useRef<Record<string, unknown> | undefined>(undefined)
  const fileInput = useRef<HTMLInputElement>(null)
  const stream = useRef<ConversationStream | null>(null)
  const nonce = useRef({ value: config.nonce?.nonce, at: Date.now() })
  const openRef = useRef(open)
  const panel = useRef<HTMLDivElement>(null)
  const launcher = useRef<HTMLButtonElement>(null)
  const input = useRef<HTMLTextAreaElement>(null)
  const log = useRef<HTMLDivElement>(null)
  openRef.current = open

  const hasToken = !!saved.current.token
  const askName = !conv && !hasToken && config.require_name
  const askEmail = !conv && !hasToken && config.require_email

  const show = (v: boolean) => {
    setOpen(v)
    if (v) setUnread(0)
    emit(v ? 'open' : 'close')
  }

  useEffect(() => {
    bind({ open: () => show(true), close: () => show(false), identify, setAttributes })
    if (location.hash.startsWith('#posta-chat=')) show(true)
    const s = saved.current
    if (s.token) api.token = s.token
    if (s.token && s.conversation) follow(s.conversation, 0)
    return () => stream.current?.stop()
  }, [])

  useEffect(() => {
    if (open) {
      input.current?.focus()
      log.current?.scrollTo({ top: log.current.scrollHeight })
    }
  }, [open])

  useEffect(() => {
    log.current?.scrollTo({ top: log.current.scrollHeight })
  }, [messages.length, agentTyping])

  // Tell the agents what the visitor has seen: the newest message, while the
  // panel is open and the tab visible.
  useEffect(() => {
    if (!open || !conv) return
    const mark = () => {
      if (document.visibilityState !== 'visible') return
      const newest = messages.reduce((n, m) => (m.author === 'agent' && m.seq > n ? m.seq : n), 0)
      if (newest > lastReadSent.current) {
        lastReadSent.current = newest
        api.read(conv.id, newest).catch(() => {})
      }
    }
    mark()
    document.addEventListener('visibilitychange', mark)
    return () => document.removeEventListener('visibilitychange', mark)
  }, [open, conv, messages])

  function onType(value: string) {
    setText(value)
    if (conv && value.trim() && Date.now() - lastTypingSent.current > 3000) {
      lastTypingSent.current = Date.now()
      api.typing(conv.id).catch(() => {})
    }
  }

  // identify applies a signed identity. A visitor new to this browser who
  // verifies gets a token, and with it the conversation they had on another
  // device.
  async function identify(id: Identity) {
    identity.current = id
    try {
      const had = !!api.token
      const res = await api.identify(id)
      if (!had && res.visitor_token) {
        api.token = res.visitor_token
        saved.current = { token: api.token }
        save(widgetKey, saved.current)
        flushAttributes()
        const list = await api.conversations()
        if (list.length && !stream.current) {
          setConv(list[0])
          saved.current = { token: api.token, conversation: list[0].id }
          save(widgetKey, saved.current)
          follow(list[0].id, 0)
        }
      }
    } catch {
      // An identity that cannot be checked leaves the visitor anonymous.
    }
  }

  function setAttributes(a: Record<string, unknown>) {
    attributes.current = a
    flushAttributes()
  }

  function flushAttributes() {
    if (api.token && attributes.current) api.attributes(attributes.current).catch(() => {})
  }

  async function sendFile(file: File) {
    if (!conv) return
    const local: Local = {
      id: '', seq: 0, author: 'visitor', body: '', client_msg_id: uuid(), created_at: new Date().toISOString(),
      pending: true, attachments: [{ filename: file.name, content_type: file.type, size: file.size }],
    }
    setMessages((list) => merge(list, [local]))
    setError('')
    try {
      if (file.size > 5 * 1024 * 1024) throw new ApiError(413, t.fileTooLarge)
      const stored = await api.upload(conv.id, file, local.client_msg_id!)
      stream.current?.seen(stored.seq)
      setMessages((list) => merge(list, [stored]))
    } catch (e) {
      setMessages((list) => list.map((x) => (x.client_msg_id === local.client_msg_id ? { ...x, pending: false, failed: true } : x)))
      setError(e instanceof ApiError && e.status === 413 ? t.fileTooLarge : describe(e))
    }
  }

  function follow(id: string, since: number) {
    stream.current?.stop()
    stream.current = new ConversationStream(api, id, since, {
      message(m) {
        setMessages((list) => merge(list, [m]))
        if (m.author === 'agent') {
          setAgentTyping(null)
          if (!openRef.current) setUnread((n) => n + 1)
          emit('message', { body: m.body, agent_name: m.agent_name, created_at: m.created_at })
        }
      },
      conversation(c) {
        setConv(c)
        if (c.agent_read_seq) setAgentRead((s) => Math.max(s, c.agent_read_seq!))
      },
      typing(name) {
        setAgentTyping(name || t.someone)
        clearTimeout(typingTimer.current)
        typingTimer.current = setTimeout(() => setAgentTyping(null), 4000)
      },
      read(seq) {
        setAgentRead((s) => Math.max(s, seq))
      },
      gone() {
        saved.current = { token: saved.current.token }
        save(widgetKey, saved.current)
        setConv(null)
        setMessages([])
      },
    })
  }

  async function freshNonce(): Promise<string | undefined> {
    if (!nonce.current.value || Date.now() - nonce.current.at > NONCE_MAX_AGE_MS) {
      try {
        const n = await api.nonce()
        nonce.current = { value: n.nonce, at: Date.now() }
      } catch {
        // Start without one; the server decides.
      }
    }
    // Wait out the minimum fill time rather than be refused for typing fast.
    const wait = nonce.current.at + (config.min_fill_seconds || 0) * 1000 - Date.now()
    if (wait > 0) await new Promise((r) => setTimeout(r, wait + 100))
    return nonce.current.value
  }

  function describe(e: unknown): string {
    if (e instanceof ApiError) {
      if (e.status === 429) return t.rateLimited
      if (e.status === 400 && e.message) return e.message
    }
    return t.error
  }

  async function deliver(m: Local) {
    setBusy(true)
    setError('')
    try {
      if (!conv) {
        const res = await api.start({
          name: name.trim() || undefined,
          email: email.trim() || undefined,
          body: m.body,
          client_msg_id: m.client_msg_id!,
          nonce: await freshNonce(),
          page_url: location.href,
          identity: api.token ? undefined : identity.current,
        })
        nonce.current = { value: undefined, at: 0 }
        if (res.status !== 'started' || !res.conversation || !res.message) {
          setMessages((list) => list.map((x) => (x.client_msg_id === m.client_msg_id ? { ...x, pending: false } : x)))
          setNotice(t.received)
          return
        }
        if (res.visitor_token) {
          api.token = res.visitor_token
          flushAttributes()
        }
        saved.current = { token: api.token, conversation: res.conversation.id }
        save(widgetKey, saved.current)
        setConv(res.conversation)
        setMessages((list) => merge(list, [res.message!]))
        follow(res.conversation.id, res.message.seq)
      } else {
        const stored = await api.send(conv.id, { body: m.body, client_msg_id: m.client_msg_id!, page_url: location.href })
        stream.current?.seen(stored.seq)
        setMessages((list) => merge(list, [stored]))
      }
    } catch (e) {
      if (e instanceof ApiError && e.status === 404 && conv) {
        // The conversation is gone; the next message starts a new one.
        saved.current = { token: saved.current.token }
        save(widgetKey, saved.current)
        setConv(null)
      }
      setMessages((list) => list.map((x) => (x.client_msg_id === m.client_msg_id ? { ...x, pending: false, failed: true } : x)))
      setError(describe(e))
    } finally {
      setBusy(false)
    }
  }

  function submit(e?: Event) {
    e?.preventDefault()
    const body = text.trim()
    if (!body || busy) return
    if (askName && !name.trim()) return setError(`${t.name}: ${t.required}`)
    if (askEmail && !EMAIL.test(email.trim())) return setError(t.emailInvalid)
    if (config.max_message_bytes && new TextEncoder().encode(body).length > config.max_message_bytes) {
      return setError(t.tooLong)
    }
    const m: Local = {
      id: '',
      seq: 0,
      author: 'visitor',
      body,
      client_msg_id: uuid(),
      created_at: new Date().toISOString(),
      pending: true,
    }
    setText('')
    setNotice('')
    setMessages((list) => merge(list, [m]))
    deliver(m)
  }

  function retry(m: Local) {
    setMessages((list) => list.map((x) => (x.client_msg_id === m.client_msg_id ? { ...x, failed: false, pending: true } : x)))
    deliver(m)
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault()
      submit()
    }
  }

  // The panel is a dialog: Escape closes it from anywhere in the widget, and
  // Tab stays inside it.
  function onWidgetKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && openRef.current) {
      show(false)
      launcher.current?.focus()
    }
  }

  function onPanelKey(e: KeyboardEvent) {
    if (e.key !== 'Tab' || !panel.current) return
    const items = Array.from(
      panel.current.querySelectorAll<HTMLElement>('button,a[href],input,textarea'),
    ).filter((el) => !el.hasAttribute('disabled'))
    if (!items.length) return
    const first = items[0]
    const last = items[items.length - 1]
    const active = (panel.current.getRootNode() as ShadowRoot).activeElement
    if (e.shiftKey && active === first) {
      e.preventDefault()
      last.focus()
    } else if (!e.shiftKey && active === last) {
      e.preventDefault()
      first.focus()
    }
  }

  const title = config.title || t.title
  // "Seen" shows under the visitor's newest message once an agent has read it.
  const newestVisitor = messages.reduce((n, m) => (m.author === 'visitor' && m.seq > n ? m.seq : n), 0)
  const newestAny = messages.reduce((n, m) => Math.max(n, m.seq || 0), 0)
  const lastSeen = newestVisitor > 0 && newestVisitor === newestAny && agentRead >= newestVisitor ? newestVisitor : 0
  return (
    <div class={`pc ${config.position === 'left' ? 'left' : ''} ${open ? 'opened' : ''}`} onKeyDown={onWidgetKey}>
      {open && (
        <div
          class="panel"
          role="dialog"
          aria-label={title}
          ref={panel}
          onKeyDown={onPanelKey}
        >
          <div class="head">
            {config.avatar_url && <img src={config.avatar_url} alt="" />}
            <h2>{title}</h2>
            <button class="x" type="button" aria-label={t.close} onClick={() => show(false)}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path d="M6 6l12 12M18 6L6 18" />
              </svg>
            </button>
          </div>
          <div class="log" ref={log} aria-live="polite">
            {config.greeting && <div class="msg greet">{config.greeting}</div>}
            {!config.online && !conv && <div class="note">{t.offline}</div>}
            {messages.map((m) => (
              <Fragment key={m.client_msg_id || m.id}>
                <div class={`msg ${m.author}`}>
                  {m.author === 'agent' && m.agent_name && <span class="who">{m.agent_name}</span>}
                  {m.body && <Body text={m.body} />}
                  {m.attachments?.map((att, i) => (
                    <File_ key={i} api={api} conv={conv?.id} message={m.id} idx={i} att={att} />
                  ))}
                </div>
                {m.pending && <span class="state">{t.sending}</span>}
                {m.failed && !m.attachments?.length && (
                  <button type="button" class="state failed" onClick={() => retry(m)}>
                    {t.failed}
                  </button>
                )}
                {m.failed && !!m.attachments?.length && <span class="state failed">{t.error}</span>}
              </Fragment>
            ))}
            {lastSeen > 0 && <span class="state seen">{t.seen}</span>}
            {agentTyping && (
              <div class="note typing" aria-live="off">
                {agentTyping} {t.typing}
              </div>
            )}
            {conv?.state === 'closed' && <div class="note">{t.closed}</div>}
            {notice && <div class="note">{notice}</div>}
          </div>
          <form class="form" onSubmit={submit}>
            {(askName || askEmail) && (
              <div class="fields">
                {askName && (
                  <input
                    type="text"
                    autocomplete="name"
                    aria-label={t.name}
                    placeholder={t.name}
                    value={name}
                    maxLength={120}
                    onInput={(e) => setName((e.target as HTMLInputElement).value)}
                  />
                )}
                {askEmail && (
                  <input
                    type="email"
                    autocomplete="email"
                    aria-label={t.email}
                    placeholder={t.email}
                    value={email}
                    maxLength={254}
                    onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
                  />
                )}
              </div>
            )}
            <div class="row">
              {config.attachments && (
                <>
                  <input
                    ref={fileInput}
                    type="file"
                    accept={ACCEPT}
                    hidden
                    onChange={(e) => {
                      const input = e.target as HTMLInputElement
                      const file = input.files?.[0]
                      input.value = ''
                      if (file) sendFile(file)
                    }}
                  />
                  <button
                    type="button"
                    class="clip"
                    aria-label={conv ? t.attach : t.attachFirst}
                    title={conv ? t.attach : t.attachFirst}
                    disabled={!conv}
                    onClick={() => fileInput.current?.click()}
                  >
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
                      <path d="M21 11.5l-8.6 8.6a5 5 0 0 1-7.1-7.1l8.6-8.6a3.5 3.5 0 0 1 5 5l-8.6 8.6a2 2 0 0 1-2.8-2.8l7.9-7.9" />
                    </svg>
                  </button>
                </>
              )}
              <textarea
                ref={input}
                rows={1}
                aria-label={t.placeholder}
                placeholder={t.placeholder}
                value={text}
                onInput={(e) => onType((e.target as HTMLTextAreaElement).value)}
                onKeyDown={onKey}
              />
              <button class="send" type="submit" disabled={busy || !text.trim()}>
                {t.send}
              </button>
            </div>
            {error && (
              <div class="err" role="alert">
                {error}
              </div>
            )}
          </form>
          <div class="foot">
            <a href="https://goposta.dev/?ref=chat-widget" target="_blank" rel="noopener">
              {t.powered}
              <svg class="mk" viewBox="78 96 356 320" width="14" height="13" aria-hidden="true" focusable="false">
                <path
                  fill="currentColor"
                  fill-rule="evenodd"
                  d="M168,96 H344 A20,20 0 0 1 364,116 V272 A20,20 0 0 1 344,292 H168 A20,20 0 0 1 148,272 V116 A20,20 0 0 1 168,96 Z M187,149 H325 A11,11 0 0 1 336,160 V160 A11,11 0 0 1 325,171 H187 A11,11 0 0 1 176,160 V160 A11,11 0 0 1 187,149 Z M187,209 H285 A11,11 0 0 1 296,220 V220 A11,11 0 0 1 285,231 H187 A11,11 0 0 1 176,220 V220 A11,11 0 0 1 187,209 Z M78,250 L256,377 L434,250 L434,380 A36,36 0 0 1 398,416 L114,416 A36,36 0 0 1 78,380 Z"
                />
              </svg>
              <strong>Posta</strong>
            </a>
          </div>
        </div>
      )}
      <button
        ref={launcher}
        class="launcher"
        type="button"
        aria-label={open ? t.close : t.open}
        aria-expanded={open}
        onClick={() => show(!open)}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          {open ? <path d="M6 9l6 6 6-6" /> : <path d="M21 12a8 8 0 0 1-11.8 7L4 20l1-5.2A8 8 0 1 1 21 12z" />}
        </svg>
        {unread > 0 && !open && <span class="badge">{unread}</span>}
        {unread > 0 && !open && <span class="sr">{unread} new</span>}
      </button>
    </div>
  )
}
