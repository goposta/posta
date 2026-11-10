import { render } from 'preact'
import { App, Controls, OpenTarget, Preview, WidgetEvent } from './App'
import { Api, Identity, WidgetConfig } from './api'
import { strings } from './i18n'
import { load } from './storage'
import { accentSheet, adopt, css, headerSheet, launcherSheet } from './styles'

type Listener = (payload?: unknown) => void
type Event = WidgetEvent

const TARGETS = ['home', 'messages', 'help', 'new']

interface PreviewOptions {
  view?: Preview['view']
  lang?: string
}

interface PostaChatAPI {
  preview(el: HTMLElement, config: WidgetConfig, options?: PreviewOptions): void
  open(target?: OpenTarget): void
  close(): void
  unread(): number
  identify(identity: Identity): void
  setAttributes(attributes: Record<string, unknown>): void
  show(): void
  hide(): void
  on(event: Event, fn: Listener): () => void
}

declare global {
  interface Window {
    PostaChat?: PostaChatAPI
    // Settings a page can set before the script loads.
    PostaChatSettings?: { identity?: Identity; attributes?: Record<string, unknown> }
  }
}

const roots = new WeakMap<HTMLElement, ShadowRoot>()

function sample(t: ReturnType<typeof strings>): Pick<Preview, 'messages' | 'list'> {
  const now = new Date().toISOString()
  return {
    messages: [
      { id: 'p1', seq: 1, author: 'visitor', body: t.previewQuestion, created_at: now },
      { id: 'p2', seq: 2, author: 'agent', agent_name: 'Jane', body: t.previewAnswer, created_at: now },
    ],
    list: [{
      id: 'p', state: 'open', last_seq: 2, last_message_at: now, created_at: now,
      unread: 1, preview: t.previewAnswer, last_author: 'agent',
    }],
  }
}

// Renders from a config object with no network calls, so the dashboard can show the real widget while it is edited.
function preview(base: string) {
  return (el: HTMLElement, config: WidgetConfig, options: PreviewOptions = {}) => {
    let root = roots.get(el)
    if (!root) {
      root = el.attachShadow({ mode: 'open' })
      roots.set(el, root)
    }
    adopt(root, [css, accentSheet(config.accent_color), headerSheet(config.home?.header_color)])
    const t = strings(options.lang)
    const view = options.view || (config.home?.enabled ? 'home' : 'thread')
    render(
      <App
        key={`${view}:${config.theme}:${config.home?.enabled}`}
        api={new Api(base, 'preview')}
        widgetKey="preview"
        config={config}
        t={t}
        preview={{ view, ...sample(t) }}
        emit={() => {}}
        bind={() => {}}
      />,
      root,
    )
  }
}

// Mounted in a shadow root so the page's CSS cannot reach in and ours cannot leak out.
;(function () {
  if (window.PostaChat) return
  const script =
    (document.currentScript as HTMLScriptElement | null) ||
    document.querySelector<HTMLScriptElement>('script[data-posta-widget]')
  const base = new URL(script?.src || location.href, location.href).origin
  const key = script?.dataset.postaWidget
  if (!script || !key) {
    window.PostaChat = { preview: preview(base) } as PostaChatAPI
    return
  }

  const listeners: Record<Event, Listener[]> = { message: [], open: [], close: [], unread: [] }
  let ctl: Controls | null = null
  let host: HTMLElement | null = null
  let pendingOpen: OpenTarget | true | false = false
  const preset = window.PostaChatSettings || {}
  let pendingIdentity: Identity | undefined = preset.identity
  let pendingAttributes: Record<string, unknown> | undefined = preset.attributes

  window.PostaChat = {
    preview: preview(base),
    open: (target) => {
      const t = target && TARGETS.includes(target) ? target : undefined
      if (ctl) ctl.open(t)
      else pendingOpen = t || true
    },
    close: () => ctl?.close(),
    unread: () => ctl?.unread() || 0,
    show: () => host?.removeAttribute('hidden'),
    hide: () => host?.setAttribute('hidden', ''),
    identify: (identity) => (ctl ? ctl.identify(identity) : (pendingIdentity = identity)),
    setAttributes: (attributes) => (ctl ? ctl.setAttributes(attributes) : (pendingAttributes = attributes)),
    on(event, fn) {
      listeners[event]?.push(fn)
      return () => {
        listeners[event] = listeners[event].filter((f) => f !== fn)
      }
    },
  }

  const emit = (event: Event, payload?: unknown) => {
    for (const fn of listeners[event]) {
      try {
        fn(payload)
      } catch {
        // A listener's error is the site's, not ours.
      }
    }
  }

  async function mount() {
    const api = new Api(base, key!)
    // A returning visitor's token lets /config greet them by name and keep a hidden widget reachable.
    api.token = load(key!).token
    let config
    try {
      config = await api.config()
    } catch {
      return
    }
    if (!config?.enabled) return
    host = document.createElement('div')
    host.setAttribute('data-posta-chat', '')
    document.body.appendChild(host)
    const root = host.attachShadow({ mode: 'open' })
    adopt(root, [css, accentSheet(config.accent_color), headerSheet(config.home?.header_color), launcherSheet(config.launcher, config.position === 'left')])
    render(
      <App
        api={api}
        widgetKey={key!}
        config={config}
        t={strings()}
        emit={emit}
        bind={(c) => {
          ctl = c
          if (pendingIdentity) c.identify(pendingIdentity)
          if (pendingAttributes) c.setAttributes(pendingAttributes)
          if (pendingOpen) c.open(pendingOpen === true ? undefined : pendingOpen)
        }}
      />,
      root,
    )
  }

  // Lets a page's own "Contact us" link open the chat without writing any script.
  document.addEventListener('click', (e) => {
    const el = (e.target as Element | null)?.closest?.('[data-posta-chat-open]')
    if (!el) return
    e.preventDefault()
    window.PostaChat?.open((el.getAttribute('data-posta-chat-open') || undefined) as OpenTarget | undefined)
  })

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mount)
  else mount()
})()
