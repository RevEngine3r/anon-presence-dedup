/**
 * UUID v4 bootstrap.
 * Reads from localStorage; generates and persists if absent.
 * This value is stable for the lifetime of the browser installation.
 */
export function getClientId(): string {
  const key = 'client_id'
  let id = localStorage.getItem(key)
  if (!id) {
    id = crypto.randomUUID()
    localStorage.setItem(key, id)
  }
  return id
}

export const CLIENT_ID = getClientId()
