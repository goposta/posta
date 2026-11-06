import { Message } from './api'

// Local is a message as the widget shows it, including its own messages
// that are still on their way or failed to send.
export interface Local extends Message {
  pending?: boolean
  failed?: boolean
}

// merge adds messages to the thread once each: a stored message replaces the
// pending copy with the same client ID, and a sequence number seen before is
// ignored. The thread stays ordered by sequence, pending messages last.
export function merge(list: Local[], incoming: Local[]): Local[] {
  let out = list.slice()
  for (const m of incoming) {
    if (m.seq && out.some((x) => x.seq === m.seq)) continue
    if (m.client_msg_id) {
      const i = out.findIndex((x) => x.client_msg_id === m.client_msg_id)
      if (i >= 0) {
        out[i] = m
        continue
      }
    }
    out.push(m)
  }
  out = out.sort((a, b) => (a.seq || Infinity) - (b.seq || Infinity))
  return out
}
