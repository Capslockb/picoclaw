import { atom } from "jotai"

export type GatewayState =
  | "running"
  | "starting"
  | "stopped"
  | "error"
  | "unknown"

export interface GatewayStoreState {
  status: GatewayState
  canStart: boolean
}

// Global atom for gateway state
export const gatewayAtom = atom<GatewayStoreState>({
  status: "unknown",
  canStart: true,
})
