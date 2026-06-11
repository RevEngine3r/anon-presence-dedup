import { useState } from 'react'
import { useWebSocket, WSMessage } from './useWebSocket'
import { useViewTracker } from './useViewTracker'
import { recordReaction } from './api'

// Minimal demo — replace with your real message list.
const DEMO_MESSAGES = [
  { id: 1, text: 'Hello, world! 👋' },
  { id: 2, text: 'This is a second message.' },
  { id: 3, text: 'Scroll down to trigger view events.' },
]

export default function App() {
  const [online, setOnline] = useState<number | null>(null)

  useWebSocket((msg: WSMessage) => {
    if (msg.type === 'PRESENCE_UPDATE') setOnline(msg.online)
  })

  return (
    <div style={{ maxWidth: 600, margin: '0 auto', padding: 24 }}>
      <header style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 20, fontWeight: 700 }}>Anon Presence Demo</h1>
        {online !== null && (
          <p style={{ color: '#555', fontSize: 14, marginTop: 4 }}>
            🟢 {online} user{online !== 1 ? 's' : ''} online
          </p>
        )}
      </header>

      {DEMO_MESSAGES.map((msg) => (
        <MessageCard key={msg.id} id={msg.id} text={msg.text} />
      ))}
    </div>
  )
}

function MessageCard({ id, text }: { id: number; text: string }) {
  const ref = useViewTracker(id)

  const react = (emoji: string) => {
    recordReaction(id, emoji).catch(() => {})
  }

  return (
    <div
      ref={ref as React.RefObject<HTMLDivElement>}
      style={{
        background: '#fff',
        border: '1px solid #e0e0e0',
        borderRadius: 8,
        padding: 16,
        marginBottom: 16,
      }}
    >
      <p style={{ marginBottom: 12 }}>{text}</p>
      <div style={{ display: 'flex', gap: 8 }}>
        {['👍', '❤️', '😂', '😮'].map((emoji) => (
          <button
            key={emoji}
            onClick={() => react(emoji)}
            style={{
              fontSize: 20,
              background: 'none',
              border: '1px solid #ddd',
              borderRadius: 6,
              padding: '2px 8px',
              cursor: 'pointer',
            }}
          >
            {emoji}
          </button>
        ))}
      </div>
    </div>
  )
}
