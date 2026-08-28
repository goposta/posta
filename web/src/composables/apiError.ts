export function apiMessage(e: any, fallback: string): string {
  const data = e?.response?.data
  const message = data?.error?.message || data?.error?.error || data?.message
  return typeof message === 'string' && message.trim() ? message : fallback
}
