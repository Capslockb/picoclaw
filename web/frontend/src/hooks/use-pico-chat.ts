import dayjs from "dayjs"
import { useAtomValue } from "jotai"
import { useCallback, useEffect, useRef, useState } from "react"

import { getPicoToken } from "@/api/pico"
import { getSessionHistory } from "@/api/sessions"
import { gatewayAtom } from "@/store"

// Pico Protocol message types
interface PicoMessage {
  type: string
  id?: string
  session_id?: string
  timestamp?: number
  payload?: Record<string, unknown>
}

export interface ChatMessage {
  id: string
  role: "user" | "assistant"
  content: string
  timestamp: number | string
}

type ConnectionState = "disconnected" | "connecting" | "connected" | "error"

function generateSessionId(): string {
  return crypto.randomUUID()
}

// Helper to format message timestamps
export function formatMessageTime(dateRaw: number | string | Date): string {
  const date = dayjs(dateRaw)
  const now = dayjs()

  const isToday = date.isSame(now, "day")
  const isThisYear = date.isSame(now, "year")

  if (isToday) {
    return date.format("LT")
  }

  // Cross-day formatting
  if (isThisYear) {
    return date.format("MMM D LT")
  }

  return date.format("ll LT")
}

export function usePicoChat() {
  const { status: gatewayState } = useAtomValue(gatewayAtom)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [connectionState, setConnectionState] =
    useState<ConnectionState>("disconnected")
  const [isTyping, setIsTyping] = useState(false)
  const [activeSessionId, setActiveSessionId] =
    useState<string>(generateSessionId)

  const wsRef = useRef<WebSocket | null>(null)
  const isConnectingRef = useRef(false)
  const msgIdCounter = useRef(0)
  const activeSessionIdRef = useRef(activeSessionId)

  // Keep ref in sync
  useEffect(() => {
    activeSessionIdRef.current = activeSessionId
  }, [activeSessionId])

  const handlePicoMessage = useCallback((msg: PicoMessage) => {
    const payload = msg.payload || {}

    switch (msg.type) {
      case "message.create": {
        const content = (payload.content as string) || ""
        const messageId = (payload.message_id as string) || `pico-${Date.now()}`
        // Use provided timestamp or current time
        const timestampRaw = msg.timestamp ? msg.timestamp * 1000 : Date.now()

        setMessages((prev) => [
          ...prev,
          {
            id: messageId,
            role: "assistant",
            content,
            timestamp: timestampRaw,
          },
        ])
        setIsTyping(false)
        break
      }

      case "message.update": {
        const content = (payload.content as string) || ""
        const messageId = payload.message_id as string
        if (!messageId) break

        setMessages((prev) =>
          prev.map((m) => (m.id === messageId ? { ...m, content } : m)),
        )
        break
      }

      case "typing.start":
        setIsTyping(true)
        break

      case "typing.stop":
        setIsTyping(false)
        break

      case "error":
        console.error("Pico error:", payload)
        setIsTyping(false)
        break

      case "pong":
        // heartbeat response, ignore
        break

      default:
        console.log("Unknown pico message type:", msg.type)
    }
  }, [])

  const connect = useCallback(async () => {
    if (
      isConnectingRef.current ||
      (wsRef.current &&
        (wsRef.current.readyState === WebSocket.OPEN ||
          wsRef.current.readyState === WebSocket.CONNECTING))
    ) {
      return
    }

    isConnectingRef.current = true
    setConnectionState("connecting")

    try {
      const { token, ws_url } = await getPicoToken()

      if (!token) {
        console.error("No pico token available")
        setConnectionState("error")
        isConnectingRef.current = false
        return
      }

      // If the backend returns a localhost URL but we are accessing it via a LAN IP
      // (e.g., from a mobile device during dev), rewrite the hostname to match.
      let finalWsUrl = ws_url
      try {
        const parsedUrl = new URL(ws_url)
        const isLocalHost =
          parsedUrl.hostname === "localhost" ||
          parsedUrl.hostname === "127.0.0.1" ||
          parsedUrl.hostname === "0.0.0.0"
        const isBrowserLocal =
          window.location.hostname === "localhost" ||
          window.location.hostname === "127.0.0.1"

        if (isLocalHost && !isBrowserLocal) {
          parsedUrl.hostname = window.location.hostname
          finalWsUrl = parsedUrl.toString()
        }
      } catch (e) {
        console.warn("Could not parse ws_url:", e)
      }

      // Build WebSocket URL with session_id
      const sessionId = activeSessionIdRef.current
      const url = `${finalWsUrl}?token=${encodeURIComponent(token)}&session_id=${encodeURIComponent(sessionId)}`
      const socket = new WebSocket(url)

      socket.onopen = () => {
        setConnectionState("connected")
        isConnectingRef.current = false
      }

      socket.onmessage = (event) => {
        try {
          const msg: PicoMessage = JSON.parse(event.data)
          handlePicoMessage(msg)
        } catch {
          console.warn("Non-JSON message from pico:", event.data)
        }
      }

      socket.onclose = () => {
        setConnectionState("disconnected")
        wsRef.current = null
        isConnectingRef.current = false
      }

      socket.onerror = () => {
        setConnectionState("error")
        isConnectingRef.current = false
      }

      wsRef.current = socket
    } catch (err) {
      console.error("Failed to connect to pico:", err)
      setConnectionState("error")
      isConnectingRef.current = false
    }
  }, [handlePicoMessage])

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setConnectionState("disconnected")
    isConnectingRef.current = false
  }, [])

  // Auto connect/disconnect based on gateway state
  useEffect(() => {
    // Wrap in setTimeout to avoid React calling setState synchronously during render
    const timerId = setTimeout(() => {
      if (gatewayState === "running") {
        connect()
      } else {
        disconnect()
      }
    }, 0)

    return () => clearTimeout(timerId)
  }, [gatewayState, connect, disconnect])

  // Cleanup on unmount
  useEffect(() => {
    return () => disconnect()
  }, [disconnect])

  const sendMessage = useCallback((content: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      console.warn("WebSocket not connected")
      return
    }

    const id = `msg-${++msgIdCounter.current}-${Date.now()}`
    const timestampRaw = Date.now()

    // Add user message to local state
    setMessages((prev) => [
      ...prev,
      { id, role: "user", content, timestamp: timestampRaw },
    ])

    // Show typing indicator immediately
    setIsTyping(true)

    // Send via Pico Protocol
    const picoMsg: PicoMessage = {
      type: "message.send",
      id,
      payload: { content },
    }
    wsRef.current.send(JSON.stringify(picoMsg))
  }, [])

  // Switch to a historical session
  const switchSession = useCallback(
    async (sessionId: string) => {
      // Disconnect current WebSocket
      disconnect()

      // Set new session ID
      setActiveSessionId(sessionId)
      setIsTyping(false)

      // Load history from backend
      try {
        const detail = await getSessionHistory(sessionId)
        // Set all history messages timestamp from the session updated time as fallback,
        // since currently the backend doesn't return per-message timestamp in the history API.
        // We'll use the session's updated time for now.
        const fallbackTime = detail.updated

        setMessages(
          detail.messages.map((m, i) => ({
            id: `hist-${i}-${Date.now()}`,
            role: m.role as "user" | "assistant",
            content: m.content,
            timestamp: fallbackTime,
          })),
        )
      } catch (err) {
        console.error("Failed to load session history:", err)
        setMessages([])
      }

      // Reconnect with new session ID (will use the updated ref)
      // Small delay to ensure state has settled
      setTimeout(() => {
        if (gatewayState === "running") {
          connect()
        }
      }, 100)
    },
    [disconnect, connect, gatewayState],
  )

  // Start a new empty chat
  const newChat = useCallback(() => {
    if (messages.length === 0) {
      return
    }

    disconnect()
    const newId = generateSessionId()
    setActiveSessionId(newId)
    setMessages([])
    setIsTyping(false)

    // Reconnect with the fresh session
    setTimeout(() => {
      if (gatewayState === "running") {
        connect()
      }
    }, 100)
  }, [disconnect, connect, gatewayState, messages.length])

  return {
    messages,
    connectionState,
    isTyping,
    activeSessionId,
    sendMessage,
    switchSession,
    newChat,
  }
}
