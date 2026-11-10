import { render } from 'preact'
import { App, Controls, OpenTarget, WidgetEvent } from './App'
import { Api, Identity } from './api'
import { strings } from './i18n'
import { accentSheet, adopt, css, headerSheet } from './styles'

type Listener = (payload?: unknown) => void
type Event = WidgetEvent

const TARGETS = ['home', 'messages', 'help', 'new']

interface PostaChatAPI {
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

// Mounted in a shadow root so the page's CSS cannot reach in and ours cannot leak out.
;(function () {
  if (window.PostaChat) return
  const script =
    (document.currentScript as HTMLScriptElement | null) ||
    document.querySelector<HTMLScriptElement>('script[data-posta-widget]')
  const key = script?.dataset.postaWidget
  if (!script || !key) return
  const base = new URL(script.src, location.href).origin

  const listeners: Record<Event, Listener[]> = { message: [], open: [], close: [], unread: [] }
  let ctl: Controls | null = null
  let host: HTMLElement | null = null
  let pendingOpen: OpenTarget | true | false = false
  const preset = window.PostaChatSettings || {}
  let pendingIdentity: Identity | undefined = preset.identity
  let pendingAttributes: Record<string, unknown> | undefined = preset.attributes

  window.PostaChat = {
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
    adopt(root, [css, accentSheet(config.accent_color), headerSheet(config.home?.header_color)])
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
