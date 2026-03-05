import { atom } from "jotai"

export type GatewayState =
  | "running"
  | "starting"
  | "stopped"
  | "error"
  | "unknown"

export interface GatewayStoreState {
  isInitialized: boolean
  status: GatewayState
}

// Global atom for gateway state
export const gatewayAtom = atom<GatewayStoreState>({
  isInitialized: false,
  status: "unknown",
})
