// Read with fetch, not EventSource, so the visitor token goes in an Authorization header instead of the URL.

export interface SSEEvent {
  id?: string
  event: string
  data: string
}

export class SSEParser {
  private buffer = ''
  private data: string[] = []
  private event = ''
  private id: string | undefined

  // push feeds a chunk of the stream and returns the events it completed.
  push(chunk: string): SSEEvent[] {
    this.buffer += chunk
    const out: SSEEvent[] = []
    let nl: number
    while ((nl = this.nextLineEnd()) >= 0) {
      let line = this.buffer.slice(0, nl)
      const skip = this.buffer[nl] === '\r' && this.buffer[nl + 1] === '\n' ? 2 : 1
      this.buffer = this.buffer.slice(nl + skip)
      if (line === '') {
        if (this.data.length) {
          out.push({ id: this.id, event: this.event || 'message', data: this.data.join('\n') })
        }
        this.data = []
        this.event = ''
        continue
      }
      if (line.startsWith(':')) continue
      const colon = line.indexOf(':')
      const field = colon < 0 ? line : line.slice(0, colon)
      let value = colon < 0 ? '' : line.slice(colon + 1)
      if (value.startsWith(' ')) value = value.slice(1)
      line = ''
      switch (field) {
        case 'data':
          this.data.push(value)
          break
        case 'event':
          this.event = value
          break
        case 'id':
          if (!value.includes('\0')) this.id = value
          break
      }
    }
    return out
  }

  private nextLineEnd(): number {
    const n = this.buffer.indexOf('\n')
    const r = this.buffer.indexOf('\r')
    if (r >= 0 && (n < 0 || r < n)) {
      // A lone \r at the end of a chunk may be the first half of \r\n.
      if (r === this.buffer.length - 1) return n
      return r
    }
    return n
  }
}

// Exponential from 1 s, capped at 30 s, with up to 30% jitter so a server restart is not met by every widget at the same instant.
export function backoff(attempt: number, random: () => number = Math.random): number {
  const base = Math.min(30000, 1000 * 2 ** Math.min(attempt, 5))
  return Math.round(base * (1 - 0.3 * random()))
}
