/**
 * UUID v4 bootstrap.
 * Reads from localStorage; generates and persists if absent.
 * Falls back to a manual RFC-4122 v4 generator when crypto.randomUUID
 * is unavailable (HTTP non-secure contexts, older browsers).
 */

function generateUUID(): string {
  // Modern browsers on HTTPS / localhost
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  // Fallback: manual v4 UUID via crypto.getRandomValues or Math.random
  const getBytes = (): number[] => {
    if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
      return Array.from(crypto.getRandomValues(new Uint8Array(16)))
    }
    return Array.from({ length: 16 }, () => Math.floor(Math.random() * 256))
  }
  const b = getBytes()
  b[6] = (b[6] & 0x0f) | 0x40  // version 4
  b[8] = (b[8] & 0x3f) | 0x80  // variant RFC 4122
  const hex = b.map(x => x.toString(16).padStart(2, '0'))
  return [
    hex.slice(0, 4).join(''),
    hex.slice(4, 6).join(''),
    hex.slice(6, 8).join(''),
    hex.slice(8, 10).join(''),
    hex.slice(10, 16).join(''),
  ].join('-')
}

export function getClientId(): string {
  const key = 'client_id'
  let id = localStorage.getItem(key)
  if (!id) {
    id = generateUUID()
    localStorage.setItem(key, id)
  }
  return id
}

export const CLIENT_ID = getClientId()
