import { useEffect, useRef } from 'react'
import { recordView } from './api'

/**
 * Tracks whether a DOM element has entered the viewport.
 * Sends a view event exactly once per (messageId, page load).
 *
 * Uses a module-level set so the seen state persists across re-renders
 * and survives React Strict Mode double-invocations.
 */
const seenViews = new Set<number>()

export function useViewTracker(messageId: number) {
  const ref = useRef<HTMLElement | null>(null)

  useEffect(() => {
    const el = ref.current
    if (!el || seenViews.has(messageId)) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          if (!seenViews.has(messageId)) {
            seenViews.add(messageId)
            recordView(messageId).catch(() => {})
          }
          observer.disconnect()
        }
      },
      { threshold: 0.5 },
    )

    observer.observe(el)
    return () => observer.disconnect()
  }, [messageId])

  return ref
}
