import { useState } from 'preact/hooks'
import { HelpLink, Home as HomeConfig, HomeItem, Summary } from './api'
import { Strings } from './i18n'
import { opensNewTab, safeHref } from './links'
import { View } from './storage'

const Arrow = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" />
  </svg>
)

const External = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" />
  </svg>
)

const Close = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
    <path d="M6 6l12 12M18 6L6 18" />
  </svg>
)

function linkProps(url: string, newTab: boolean) {
  return opensNewTab(url, newTab) ? { target: '_blank', rel: 'noopener noreferrer' } : {}
}

export interface HomeProps {
  home: HomeConfig
  greeting?: string
  online: boolean
  canContinue: boolean
  t: Strings
  onMessage(): void
  onClose(): void
}

export function HomeView({ home, greeting, online, canContinue, t, onMessage, onClose }: HomeProps) {
  return (
    <div class="hv">
      <div class="home-head">
        <button class="x" type="button" aria-label={t.close} onClick={onClose}>
          <Close />
        </button>
        <h2 tabIndex={-1} data-vh>{greeting || t.title}</h2>
        {home.headline && <p>{home.headline}</p>}
      </div>
      <div class="items">
        {(home.items || []).map((it) => (
          <Item key={it.id} item={it} online={online} canContinue={canContinue} t={t} onMessage={onMessage} />
        ))}
      </div>
    </div>
  )
}

function Item({ item, online, canContinue, t, onMessage }: { item: HomeItem; online: boolean; canContinue: boolean; t: Strings; onMessage(): void }) {
  if (item.kind === 'message') {
    return (
      <button type="button" class="card" onClick={onMessage}>
        <span class="t">
          <span>{canContinue ? t.continueChat : item.title || t.sendMessage}</span>
          <Arrow />
        </span>
        {!online && !canContinue && <span class="s">{t.replyByEmail}</span>}
      </button>
    )
  }
  const href = safeHref(item.url)
  if (item.kind === 'link') {
    if (!href) return null
    const props = linkProps(href, item.new_tab !== false)
    return (
      <a class="card" href={href} {...props}>
        <span class="t">
          <span>{item.title}</span>
          <External />
        </span>
        {props.target && <span class="sr">{t.newTab}</span>}
      </a>
    )
  }
  const props = href ? linkProps(href, item.new_tab !== false) : {}
  return (
    <div class="card">
      <span class="t">
        <span>{item.title}</span>
      </span>
      {item.body && <span class="b">{item.body}</span>}
      {href && (
        <a class="l" href={href} {...props}>
          {item.link_label || item.title} →{props.target && <span class="sr"> {t.newTab}</span>}
        </a>
      )}
    </div>
  )
}

function when(iso: string): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  const sameDay = d.toDateString() === new Date().toDateString()
  return new Intl.DateTimeFormat(undefined, sameDay ? { hour: '2-digit', minute: '2-digit' } : { day: 'numeric', month: 'short' }).format(d)
}

export interface MessagesProps {
  list: Summary[] | null
  t: Strings
  onOpen(c: Summary): void
  onNew(): void
}

export function MessagesView({ list, t, onOpen, onNew }: MessagesProps) {
  return (
    <div class="hv">
      <h2 class="vh" tabIndex={-1} data-vh>{t.messages}</h2>
      {list && list.length > 0 ? (
        <ul class="list">
          {list.map((c) => (
            <li key={c.id}>
              <button type="button" class="conv" onClick={() => onOpen(c)}>
                <span class="m">
                  <span class={`p ${c.unread ? 'unread' : ''}`}>
                    {c.last_author === 'visitor' ? `${t.you}: ` : ''}
                    {c.preview || '…'}
                  </span>
                  <span class="w">
                    {when(c.last_message_at)}
                    {c.state === 'closed' ? ` · ${t.closedConv}` : ''}
                  </span>
                </span>
                {c.unread > 0 && <span class="count">{c.unread}</span>}
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <div class="empty">
          {list && <span>{t.noConversations}</span>}
          <button type="button" class="newc" onClick={onNew}>
            {t.sendMessage}
          </button>
        </div>
      )}
    </div>
  )
}

export function HelpView({ title, links, t }: { title?: string; links: HelpLink[]; t: Strings }) {
  const [q, setQ] = useState('')
  const needle = q.trim().toLowerCase()
  const shown = needle
    ? links.filter((l) => `${l.title} ${l.description || ''}`.toLowerCase().includes(needle))
    : links
  return (
    <div class="hv">
      <h2 class="vh" tabIndex={-1} data-vh>{title || t.help}</h2>
      {links.length > 6 && (
        <input
          class="filter"
          type="search"
          aria-label={t.search}
          placeholder={t.search}
          value={q}
          onInput={(e) => setQ((e.target as HTMLInputElement).value)}
        />
      )}
      <ul class="list">
        {shown.map((l) => {
          const href = safeHref(l.url)
          if (!href) return null
          const props = linkProps(href, true)
          return (
            <li key={l.id}>
              <a class="link" href={href} {...props}>
                <span class="m">
                  <span>{l.title}</span>
                  {l.description && <span class="w">{l.description}</span>}
                </span>
                <External />
                {props.target && <span class="sr">{t.newTab}</span>}
              </a>
            </li>
          )
        })}
      </ul>
      {!shown.length && <div class="empty">{t.noResults}</div>}
    </div>
  )
}

const ICONS: Record<Exclude<View, 'thread'>, string> = {
  home: 'M3 11l9-8 9 8M5 9.5V20h5v-6h4v6h5V9.5',
  messages: 'M21 12a8 8 0 0 1-11.8 7L4 20l1-5.2A8 8 0 1 1 21 12z',
  help: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.6v.6M12 17h.01',
}

export function TabBar({ tabs, view, unread, t, onSelect }: { tabs: Exclude<View, 'thread'>[]; view: View; unread: number; t: Strings; onSelect(v: View): void }) {
  function onKey(e: KeyboardEvent) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    const focused = (e.target as HTMLElement).getAttribute('data-tab') as Exclude<View, 'thread'> | null
    const i = tabs.indexOf(focused || (view as Exclude<View, 'thread'>))
    const next = tabs[(i + (e.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length]
    e.preventDefault()
    onSelect(next)
    const bar = e.currentTarget as HTMLElement
    requestAnimationFrame(() => bar.querySelector<HTMLElement>(`[data-tab=${next}]`)?.focus())
  }
  return (
    <div class="tabs" role="tablist" aria-label={t.title} onKeyDown={onKey}>
      {tabs.map((v) => (
        <button
          key={v}
          type="button"
          role="tab"
          class="tab"
          data-tab={v}
          aria-selected={view === v}
          tabIndex={view === v ? 0 : -1}
          onClick={() => onSelect(v)}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d={ICONS[v]} />
          </svg>
          {t[v]}
          {v === 'messages' && unread > 0 && (
            <>
              <span class="count" aria-hidden="true">{unread}</span>
              <span class="sr">({unread})</span>
            </>
          )}
        </button>
      ))}
    </div>
  )
}
