import { describe, expect, it } from 'vitest'
import { linkify } from '../linkify'
import { SSEParser, backoff } from '../sse'
import { Local, merge } from '../thread'

describe('SSEParser', () => {
  it('parses events split across chunks', () => {
    const p = new SSEParser()
    expect(p.push('id: 3\nevent: chat.mes')).toEqual([])
    expect(p.push('sage\ndata: {"seq":3}\n')).toEqual([])
    expect(p.push('\n')).toEqual([{ id: '3', event: 'chat.message', data: '{"seq":3}' }])
  })

  it('handles CRLF, including a CR at the end of a chunk', () => {
    const p = new SSEParser()
    expect(p.push('event: a\r\ndata: 1\r')).toEqual([])
    expect(p.push('\n\r\n')).toEqual([{ id: undefined, event: 'a', data: '1' }])
  })

  it('joins multi-line data and ignores comments and pings', () => {
    const p = new SSEParser()
    const out = p.push(': ping\n\ndata: one\ndata: two\n\n')
    expect(out).toEqual([{ id: undefined, event: 'message', data: 'one\ntwo' }])
  })

  it('keeps the last id across events', () => {
    const p = new SSEParser()
    const out = p.push('id: 7\ndata: a\n\ndata: b\n\n')
    expect(out.map((e) => e.id)).toEqual(['7', '7'])
  })
})

describe('backoff', () => {
  it('grows to a 30 second cap', () => {
    const none = () => 0
    expect(backoff(0, none)).toBe(1000)
    expect(backoff(1, none)).toBe(2000)
    expect(backoff(4, none)).toBe(16000)
    expect(backoff(5, none)).toBe(30000)
    expect(backoff(50, none)).toBe(30000)
  })

  it('jitters down by at most 30%', () => {
    expect(backoff(5, () => 1)).toBe(21000)
  })
})

describe('linkify', () => {
  it('links http, https and mailto only', () => {
    const parts = linkify('see https://example.com/a?b=1, or mailto:hi@example.com. javascript:alert(1)')
    expect(parts.filter((p) => p.href).map((p) => p.href)).toEqual([
      'https://example.com/a?b=1',
      'mailto:hi@example.com',
    ])
    expect(parts.map((p) => p.text).join('')).toBe(
      'see https://example.com/a?b=1, or mailto:hi@example.com. javascript:alert(1)',
    )
  })

  it('keeps markup as text', () => {
    const parts = linkify('<img src=x onerror=alert(1)>')
    expect(parts).toEqual([{ text: '<img src=x onerror=alert(1)>' }])
  })
})

describe('merge', () => {
  const msg = (seq: number, extra: Partial<Local> = {}): Local => ({
    id: `m${seq}`,
    seq,
    author: 'visitor',
    body: `b${seq}`,
    created_at: '',
    ...extra,
  })

  it('replaces a pending message with the stored one', () => {
    const pending = msg(0, { client_msg_id: 'c1', pending: true })
    const out = merge([msg(1), pending], [msg(2, { client_msg_id: 'c1' })])
    expect(out.map((m) => [m.seq, !!m.pending])).toEqual([
      [1, false],
      [2, false],
    ])
  })

  it('ignores a sequence number it already has', () => {
    const out = merge([msg(1), msg(2)], [msg(2), msg(1)])
    expect(out.map((m) => m.seq)).toEqual([1, 2])
  })

  it('orders by sequence and keeps pending messages last', () => {
    const out = merge([msg(0, { client_msg_id: 'p', pending: true })], [msg(3), msg(1)])
    expect(out.map((m) => m.seq)).toEqual([1, 3, 0])
  })
})
