import { Message } from './api'

// Local includes the widget's own messages still on their way or failed to send.
export interface Local extends Message {
  pending?: boolean
  failed?: boolean
}

// A stored message replaces the pending copy with the same client ID, a seq seen before is ignored, and pending messages stay last.
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
