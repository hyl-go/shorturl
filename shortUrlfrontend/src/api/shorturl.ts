import axios, { type AxiosError } from 'axios'

/**
 * 开发时走 Vite 代理：请求 `/api/...` 会 rewrite 为后端无 `/api` 前缀的真实路径。
 * 生产可配置为同域 `/api` 或完整网关地址。
 */
const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

const client = axios.create({
  baseURL: API_BASE,
  timeout: 15000
})

/** 与后端 Admin.ApiToken 一致；仅发往 /stats、/analyze、/links* */
const adminApiToken = (import.meta.env.VITE_ADMIN_API_TOKEN as string | undefined)?.trim()
if (adminApiToken) {
  client.interceptors.request.use((config) => {
    const u = config.url ?? ''
    if (
      u.startsWith('/stats') ||
      u.startsWith('/analyze') ||
      u.startsWith('/links') ||
      u.startsWith('/admin/')
    ) {
      config.headers = config.headers ?? {}
      ;(config.headers as Record<string, string>)['X-Admin-Token'] = adminApiToken
    }
    return config
  })
}

export interface ConvertPayload {
  longURL: string
  customShortURL?: string
  /** 快捷：30m | 1h | 1d | 7d */
  expirePreset?: string
  expireAfterValue?: number
  /** minute | hour | day | week | month | year */
  expireAfterUnit?: string
  /** RFC3339，兼容旧用法 */
  expireAt?: string
  enableAI?: boolean
}

export interface StatsPayload {
  shortURL: string
  startDate: string
  endDate: string
}

export interface StatsResponse {
  totalPV: number
  totalUV: number
  chartData: { date: string; pv: number; uv: number }[]
  deviceStats: { mobileRate?: number; breakdown?: { device: string; count: number }[] }
  geoStats: { country: string; city: string; count: number }[]
  geoByCountry?: { name: string; count: number }[]
  geoByRegion?: { name: string; count: number }[]
}

export interface LinkListParams {
  page?: number
  pageSize?: number
  /** 空为全部；「其他」含历史未填分类 */
  category?: string
}

/** GET /links 单条（含管理员扩展字段） */
export interface LinkListRow {
  id: number
  longURL: string
  shortURL: string
  shortPath: string
  category?: string
  safetyStatus?: string
  expireAt?: string
  createAt: string
  updateAt?: string
  pageTitle?: string
  pageDescription?: string
  aiSuggestions?: string[]
  md5?: string
}

export interface LinkUpdatePayload {
  longURL?: string
  category?: string
  noExpire?: boolean
  expirePreset?: string
  expireAfterValue?: number
  expireAfterUnit?: string
  expireAt?: string
}

/** 从 go-zero / axios 错误中解析可读文案 */
export function apiErrorMessage(e: unknown, fallback: string): string {
  const err = e as AxiosError<unknown>
  const data = err.response?.data
  if (typeof data === 'string' && data.trim()) return data.trim()
  if (data && typeof data === 'object') {
    const o = data as Record<string, unknown>
    if (typeof o.message === 'string' && o.message) return o.message
    if (typeof o.msg === 'string' && o.msg) return o.msg
  }
  if (err.message) return err.message
  return fallback
}

/**
 * 后端返回的 shortURL 字段常为「域名/路径」形式（不一定含 scheme），用于浏览器新开标签试跳。
 */
export function openShortUrlDisplay(shortURLFromApi: string): void {
  const u = shortURLFromApi.trim()
  if (!u) return
  const href = /^https?:\/\//i.test(u) ? u : `http://${u}`
  window.open(href, '_blank', 'noopener,noreferrer')
}

/** 转链（含 AI）链路较长，单独放宽超时，避免浏览器/代理先断开 */
export const convertShortUrl = (payload: ConvertPayload) =>
  client.post('/convert', payload, { timeout: 180000 })

export const getStats = (params: StatsPayload) => client.get<StatsResponse>('/stats', { params })

/** AI 报告由后端 Asynq 异步生成；本接口立即返回统计 + reportJob */
export interface AIReportDTO {
  title?: string
  summary?: string
  trends?: string[]
  anomalies?: string[]
  suggestions?: string[]
  markdown?: string
}

export interface ReportJobDTO {
  jobId: string
  status: string
}

export interface AnalyzeResponseDTO {
  statistics: StatsResponse
  aiReport?: AIReportDTO | null
  reportJob: ReportJobDTO
}

export const analyzeStats = (payload: StatsPayload) =>
  client.post<AnalyzeResponseDTO>('/analyze', payload, { timeout: 60000 })

/** 轮询异步报告任务状态 */
export const getAnalyzeReportStatus = (jobId: string) =>
  client.get<{
    status: string
    aiReport?: AIReportDTO
    error?: string
    markdownEdited?: string
  }>('/analyze/report/status', { params: { jobId } })

/** 管理端保存编辑后的 Markdown */
export const updateAnalyzeReportMarkdown = (jobId: string, markdown: string) =>
  client.put<{ ok: boolean }>('/analyze/report', { jobId, markdown })

export const listLinks = (params: LinkListParams) =>
  client.get<{ total: number; list: LinkListRow[] }>('/links', { params })

export const updateLink = (id: number, payload: LinkUpdatePayload) =>
  client.put<{ item: LinkListRow }>(`/links/${id}`, payload)

export const deleteLink = (id: number) => client.delete<{ ok: boolean }>(`/links/${id}`)

/** 库中实际出现的分类（去重），用于管理端筛选下拉 */
export const listLinkCategories = () =>
  client.get<{ categories: string[] }>('/links/categories')

/** GET /admin/performance — 主机、MySQL、Redis 快照（需 X-Admin-Token，与后端 Admin.ApiToken 一致） */
export interface PerformanceSnapshot {
  collectedAt: string
  host: {
    hostname: string
    os: string
    platform: string
    kernel: string
    uptimeSec: number
    procs: number
    bootTime: number
    goRoutines: number
  }
  cpu: { usagePercent: number; load1?: number; load5?: number; load15?: number }
  memory: { totalBytes: number; availableBytes: number; usedBytes: number; usedPercent: number }
  disk: {
    path: string
    totalBytes: number
    usedBytes: number
    freeBytes: number
    usedPercent: number
    inodesTotal: number
    inodesUsed: number
    inodesFree: number
    inodesUsedPercent: number
  }
  diskIO: { readBytes: number; writeBytes: number; readCount: number; writeCount: number; note?: string }
  mysql: {
    ok: boolean
    error?: string
    pingMs: number
    version?: string
    maxConnections: number
    threadsConnected: number
    threadsRunning: number
    questions: number
    slowQueries: number
    uptimeSec: number
    maxUsedConnections: number
  }
  redis: {
    ok: boolean
    error?: string
    pingMs: number
    redisVersion?: string
    usedMemory: number
    usedMemoryHuman?: string
    connectedClients: number
    totalCommandsProcessed: number
    instantaneousOpsPerSec: number
    keyspaceHits: number
    keyspaceMisses: number
    rdbLastSaveTime: number
    aofEnabled?: string
  }
}

export const getPerformanceSnapshot = () =>
  client.get<PerformanceSnapshot>('/admin/performance', { timeout: 30000 })
