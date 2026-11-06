// Status phrases the server sends when it has nothing specific to say; the more detailed `error.error` wins over these.
const GENERIC = new Set([
  'bad request',
  'unauthorized',
  'forbidden',
  'not found',
  'method not allowed',
  'conflict',
  'unprocessable entity',
  'too many requests',
  'internal server error',
  'bad gateway',
  'service unavailable',
  'error',
])

function usable(value: unknown): string {
  return typeof value === 'string' && value.trim() ? value.trim() : ''
}

function specific(value: unknown): string {
  const text = usable(value)
  return text && !GENERIC.has(text.toLowerCase()) ? text : ''
}

export function apiMessage(e: any, fallback: string): string {
  const data = e?.response?.data

  // A handler's own message wins; the status phrase beats the caller's fallback only because it names the failure.
  return (
    specific(data?.error?.message) ||
    specific(data?.error?.error) ||
    specific(data?.message) ||
    specific(data?.details) ||
    fallback
  )
}
