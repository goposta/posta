import { useState } from 'preact/hooks'
import { HelpLink, Home as HomeConfig, HomeItem, StatusLine, Summary } from './api'
import { backText, namesText, Strings } from './i18n'
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
  newChats: boolean
  status?: StatusLine
  t: Strings
  asset(path?: string): string | undefined
  onTrack(id: string): void
  onMessage(): void
  onClose(): void
}

function statusText(t: Strings, online: boolean, newChats: boolean, s?: StatusLine): string {
  if (online) {
    const r = s?.reply_time
    return r === 'minutes' ? t.replyMinutes : r === 'hours' ? t.replyHours : r === 'day' ? t.replyDay : ''
  }
  if (s?.next_open) return backText(t, s.next_open)
  return newChats ? t.replyByEmail : t.away
}

export function HomeView({ home, greeting, online, canContinue, newChats, status, t, asset, onTrack, onMessage, onClose }: HomeProps) {
  const logo = asset(home.logo_url)
  const line = statusText(t, online, newChats, status)
  const agents = online ? status?.agents || [] : []
  return (
    <div class="hv">
      <div class="home-head">
        <button class="x" type="button" aria-label={t.close} onClick={onClose}>
          <Close />
        </button>
        {logo && <img class="logo" src={logo} alt="" />}
        <h2 tabIndex={-1} data-vh>{greeting || t.title}</h2>
        {home.headline && <p>{home.headline}</p>}
        {agents.length > 0 && (
          <div class="agents">
            <span class="faces" aria-hidden="true">
              {agents.map((a) => (
                <span key={a} class="face">{a.slice(0, 1).toUpperCase()}</span>
              ))}
            </span>
            <span>{namesText(t, agents)}</span>
          </div>
        )}
        {line && (
          <div class={`status ${online ? 'on' : ''}`}>
            <span class="dot" aria-hidden="true" />
            {line}
          </div>
        )}
      </div>
      <div class="items">
        {(home.items || []).filter((it) => it.kind !== 'message' || newChats || canContinue).map((it) => (
          <Item key={it.id} item={it} image={asset(it.image_url)} online={online} canContinue={canContinue} t={t}
            onMessage={() => {
              onTrack(it.id)
              onMessage()
            }}
            onLink={() => onTrack(it.id)}
          />
        ))}
      </div>
    </div>
  )
}

function Item({ item, image, online, canContinue, t, onMessage, onLink }: { item: HomeItem; image?: string; online: boolean; canContinue: boolean; t: Strings; onMessage(): void; onLink(): void }) {
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
      <a class="card" href={href} {...props} onClick={onLink}>
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
      {image && <img class="img" src={image} alt="" />}
      <span class="t">
        <span>{item.title}</span>
      </span>
      {item.body && <span class="b">{item.body}</span>}
      {href && (
        <a class="l" href={href} {...props} onClick={onLink}>
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
  canStartNew: boolean
  newChats: boolean
  t: Strings
  onOpen(c: Summary): void
  onNew(): void
}

export function MessagesView({ list, canStartNew, newChats, t, onOpen, onNew }: MessagesProps) {
  return (
    <div class="hv">
      <h2 class="vh" tabIndex={-1} data-vh>{t.messages}</h2>
      {list && list.length > 0 ? (
        <>
        {canStartNew && newChats && (
          <button type="button" class="newc" onClick={onNew}>
            {t.newConversation}
          </button>
        )}
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
        </>
      ) : (
        <div class="empty">
          {list && <span>{t.noConversations}</span>}
          {newChats && (
            <button type="button" class="newc" onClick={onNew}>
              {t.sendMessage}
            </button>
          )}
        </div>
      )}
    </div>
  )
}

export function HelpView({ title, links, t, onTrack }: { title?: string; links: HelpLink[]; t: Strings; onTrack(id: string): void }) {
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
              <a class="link" href={href} {...props} onClick={() => onTrack(l.id)}>
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
