/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** axios baseURL，默认 `/api`（与 Vite 代理一致） */
  readonly VITE_API_BASE?: string
  /** 与后端 shorturl-api.yaml Admin.ApiToken 一致；不设则不带头（后端校验关闭） */
  readonly VITE_ADMIN_API_TOKEN?: string
}

import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    requiresAdmin?: boolean
    role?: string
  }
}
