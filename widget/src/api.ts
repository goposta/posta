// Every call names the widget by its public key; calls after the first message carry the visitor's bearer token.

export interface Nonce {
  nonce: string
  expires_in: number
}

export interface WidgetConfig {
  enabled: boolean
  reason?: string
  online?: boolean
  title?: string
  greeting?: string
  accent_color?: string
  position?: 'left' | 'right'
  avatar_url?: string
  home?: Home
  theme?: 'auto' | 'light' | 'dark'
  new_chats?: boolean
  status?: StatusLine
  visitor_name?: string
  launcher?: Launcher
  teaser?: Teaser
  track_clicks?: boolean
  one_open_conversation?: boolean
  require_name?: boolean
  require_email?: boolean
  max_message_bytes?: number
  attachments?: boolean
  min_fill_seconds?: number
  nonce?: Nonce
}

export interface HomeItem {
  id: string
  kind: 'message' | 'link' | 'card'
  title: string
  body?: string
  url?: string
  link_label?: string
  new_tab?: boolean
  image_url?: string
}

export interface HelpLink {
  id: string
  title: string
  description?: string
  url: string
}

export interface Home {
  enabled: boolean
  logo_url?: string
  header_color?: string
  headline?: string
  items?: HomeItem[]
  help_title?: string
  help_links?: HelpLink[]
}

export interface Launcher {
  label?: string
  icon?: 'chat' | 'help' | 'mail'
  offset_x?: number
  offset_y?: number
  hide_on_mobile?: boolean
}

export interface Teaser {
  enabled: boolean
  text?: string
  delay_seconds?: number
  paths?: string[]
}

export interface StatusLine {
  reply_time?: 'minutes' | 'hours' | 'day'
  next_open?: string
  agents?: string[]
}

export interface Message {
  id: string
  seq: number
  author: 'visitor' | 'agent' | 'system'
  agent_name?: string
  body: string
  client_msg_id?: string
  created_at: string
  attachments?: Attachment[]
}

export interface Attachment {
  filename: string
  content_type: string
  size: number
}

export interface Identity {
  user_id: string
  email: string
  name?: string
  signature: string
}

export interface Conversation {
  id: string
  state: 'open' | 'pending' | 'closed'
  last_seq: number
  agent_read_seq?: number
  last_message_at: string
  created_at: string
}

export interface Summary extends Conversation {
  unread: number
  preview: string
  last_author: 'visitor' | 'agent' | 'system' | ''
}

export interface StartResult {
  status: 'started' | 'received'
  visitor_token?: string
  conversation?: Conversation
  message?: Message
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

export class Api {
  token?: string

  constructor(private base: string, private key: string) {}

  // Only Posta's own asset paths are turned into image URLs.
  asset(path?: string): string | undefined {
    return path && path.startsWith('/widget/v1/assets/') ? new URL(path, this.base).href : undefined
  }

  url(path: string): string {
    return `${this.base}/api/v1/chat/${encodeURIComponent(this.key)}${path}`
  }

  headers(json = false): Record<string, string> {
    const h: Record<string, string> = { Accept: 'application/json' }
    if (json) h['Content-Type'] = 'application/json'
    if (this.token) h.Authorization = `Bearer ${this.token}`
    return h
  }

  private async call<T>(method: string, path: string, body?: unknown): Promise<T> {
    const res = await fetch(this.url(path), {
      method,
      headers: this.headers(body !== undefined),
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'omit',
      mode: 'cors',
    })
    let payload: any = null
    try {
      payload = await res.json()
    } catch {
      // Empty or non-JSON body.
    }
    if (!res.ok) {
      throw new ApiError(res.status, payload?.message || payload?.error?.message || res.statusText)
    }
    return payload?.data as T
  }

  config(): Promise<WidgetConfig> {
    return this.call('GET', '/config')
  }

  nonce(): Promise<Nonce> {
    return this.call('GET', '/nonce')
  }

  start(body: {
    name?: string
    email?: string
    body: string
    client_msg_id: string
    nonce?: string
    page_url: string
    identity?: Identity
  }): Promise<StartResult> {
    return this.call('POST', '/conversations', body)
  }

  messages(conv: string, after: number): Promise<Message[]> {
    return this.call('GET', `/conversations/${encodeURIComponent(conv)}/messages?after=${after}&limit=200`)
  }

  identify(identity: Identity): Promise<{ verified: boolean; visitor_token?: string }> {
    return this.call('POST', '/identify', identity)
  }

  attributes(attributes: Record<string, unknown>): Promise<void> {
    return this.call('POST', '/attributes', { attributes })
  }

  conversations(): Promise<Summary[]> {
    return this.call('GET', '/conversations')
  }

  async upload(conv: string, file: File, clientMsgID: string): Promise<Message> {
    const form = new FormData()
    form.append('file', file)
    form.append('client_msg_id', clientMsgID)
    const headers = this.headers()
    const res = await fetch(this.url(`/conversations/${encodeURIComponent(conv)}/attachments`), {
      method: 'POST', headers, body: form, credentials: 'omit', mode: 'cors',
    })
    let payload: any = null
    try {
      payload = await res.json()
    } catch {
      // Empty body.
    }
    if (!res.ok) throw new ApiError(res.status, payload?.message || res.statusText)
    return payload.data
  }

  // An <img src> cannot carry the visitor's token, so attachments are fetched here and shown through an object URL.
  async attachment(conv: string, message: string, idx: number): Promise<Blob> {
    const res = await fetch(
      this.url(`/conversations/${encodeURIComponent(conv)}/messages/${encodeURIComponent(message)}/attachments/${idx}`),
      { headers: this.headers(), credentials: 'omit', mode: 'cors' },
    )
    if (!res.ok) throw new ApiError(res.status, res.statusText)
    return res.blob()
  }

  // keepalive lets the count go out even when the click navigates away from the page.
  event(type: 'home_item_click' | 'help_link_click', itemID: string): void {
    fetch(this.url('/events'), {
      method: 'POST', headers: this.headers(true), body: JSON.stringify({ type, item_id: itemID }),
      credentials: 'omit', mode: 'cors', keepalive: true,
    }).catch(() => {})
  }

  typing(conv: string): Promise<void> {
    return this.call('POST', `/conversations/${encodeURIComponent(conv)}/typing`)
  }

  read(conv: string, seq: number): Promise<{ seq: number }> {
    return this.call('POST', `/conversations/${encodeURIComponent(conv)}/read`, { seq })
  }

  send(conv: string, body: { body: string; client_msg_id: string; page_url: string }): Promise<Message> {
    return this.call('POST', `/conversations/${encodeURIComponent(conv)}/messages`, body)
  }
}
