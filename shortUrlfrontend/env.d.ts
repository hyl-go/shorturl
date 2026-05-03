/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** axios baseURL，默认 `/api`（与 Vite 代理一致） */
  readonly VITE_API_BASE?: string
}

import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    requiresAdmin?: boolean
    role?: string
  }
}
