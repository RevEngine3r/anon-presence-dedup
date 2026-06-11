import { useEffect, useRef } from 'react'
import { CLIENT_ID } from './clientId'

export type WSMessage =
  | { type: 'PRESENCE_UPDATE'; online: number }
  | { type: 'REACTION_UPDATE'; id: number; emoji: string; count: number }
  | { type: 'MESSAGE_UPDATE'; id: number; views: number }

type Handler = (msg: WSMessage) => void

const WS_URL = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws?client_id=${CLIENT_ID}`

export function useWebSocket(onMessage: Handler) {
  const handlerRef = useRef<Handler>(onMessage)
  handlerRef.current = onMessage

  useEffect(() => {
    return wsManager.subscribe((msg) => handlerRef.current(msg))
  }, [])
}

// ---------------------------------------------------------------------------
// Singleton WebSocket manager
// ---------------------------------------------------------------------------
type Subscriber = (msg: WSMessage) => void

const wsManager = (() => {
  let ws: WebSocket | null = null
  const subscribers = new Set<Subscriber>()

  function connect() {
    ws = new WebSocket(WS_URL)

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as WSMessage
        subscribers.forEach((fn) => fn(msg))
      } catch {
        // ignore malformed
      }
    }

    ws.onclose = () => {
      setTimeout(connect, 2000)
    }
  }

  connect()

  return {
    // Returns a void cleanup function so useEffect is happy.
    subscribe(fn: Subscriber): () => void {
      subscribers.add(fn)
      return () => { subscribers.delete(fn) }
    },
  }
})()
