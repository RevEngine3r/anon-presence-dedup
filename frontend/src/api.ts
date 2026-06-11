import { CLIENT_ID } from './clientId'

const BASE = '/api'

/** Shared headers for every HTTP request. */
function headers(extra?: Record<string, string>): HeadersInit {
  return {
    'Content-Type': 'application/json',
    'X-Client-ID': CLIENT_ID,
    ...extra,
  }
}

/** Record a view for a message (server-side dedup applies too). */
export async function recordView(messageId: number): Promise<void> {
  await fetch(`${BASE}/messages/${messageId}/view`, {
    method: 'POST',
    headers: headers(),
  })
}

/** Send a reaction emoji for a message. */
export async function recordReaction(messageId: number, emoji: string): Promise<void> {
  await fetch(`${BASE}/messages/${messageId}/react`, {
    method: 'POST',
    headers: headers(),
    body: JSON.stringify({ emoji }),
  })
}
