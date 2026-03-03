import { useCallback, useState } from "react"

import { getStatus } from "@/api/status"

export function useApiStatus() {
  const [status, setStatus] = useState<string>("Unknown")
  const [loading, setLoading] = useState(false)

  const check = useCallback(async () => {
    setLoading(true)
    try {
      const data = await getStatus()
      setStatus(data.status || "Success")
    } catch (err) {
      setStatus("Fetch failed")
      console.error(err)
    } finally {
      setLoading(false)
    }
  }, [])

  return { status, loading, check }
}
