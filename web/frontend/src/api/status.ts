// API client for the picoclaw web backend.

const BASE_URL = ""

interface StatusResponse {
  status: string
  version: string
  uptime: string
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, options)
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`)
  }
  return res.json() as Promise<T>
}

export async function getStatus(): Promise<StatusResponse> {
  return request<StatusResponse>("/api/status")
}
