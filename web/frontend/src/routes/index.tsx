import { IconMessageCircle, IconServer } from "@tabler/icons-react"
import { createFileRoute } from "@tanstack/react-router"

import { Button } from "@/components/ui/button"
import { useApiStatus } from "@/hooks/use-api-status"
import { useWebSocket } from "@/hooks/use-websocket"

export const Route = createFileRoute("/")({
  component: Index,
})

function Index() {
  const { status: apiStatus, loading, check: checkApiStatus } = useApiStatus()
  const { message: wsMessage, connect: connectWebSocket } =
    useWebSocket("/ws/chat")

  return (
    <div className="flex w-full flex-col items-center justify-center gap-10 py-20">
      <div className="bg-card w-full max-w-sm rounded-xl border p-6 shadow-sm">
        <h2 className="mb-4 text-lg font-semibold tracking-tight">
          Backend API Status
        </h2>
        <div className="flex items-center justify-between">
          <span className="text-muted-foreground flex items-center gap-2 text-sm">
            Status:{" "}
            <span className="text-foreground font-medium">{apiStatus}</span>
          </span>
          <Button
            onClick={checkApiStatus}
            size="sm"
            variant="secondary"
            disabled={loading}
          >
            <IconServer className="mr-2 h-4 w-4" /> Check
          </Button>
        </div>
      </div>

      <div className="bg-card w-full max-w-sm rounded-xl border p-6 shadow-sm">
        <h2 className="mb-4 text-lg font-semibold tracking-tight">
          WebSocket Chat
        </h2>
        <div className="flex flex-col gap-4">
          <div className="bg-muted text-muted-foreground min-h-24 rounded-md p-3 text-sm whitespace-pre-wrap">
            {wsMessage}
          </div>
          <Button
            onClick={connectWebSocket}
            variant="outline"
            className="w-full"
            disabled
          >
            <IconMessageCircle className="mr-2 h-4 w-4" /> Connect to Chat
          </Button>
        </div>
      </div>
    </div>
  )
}
