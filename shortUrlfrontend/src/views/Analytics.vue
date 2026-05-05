<script setup lang="ts">
import {
  computed,
  defineAsyncComponent,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch
} from 'vue'
import { ElMessage } from 'element-plus'
import { marked } from 'marked'
import { jsPDF } from 'jspdf'
import html2canvas from 'html2canvas'
import {
  analyzeStats,
  apiErrorMessage,
  getAnalyzeReportStatus,
  getStats,
  updateAnalyzeReportMarkdown,
  type AIReportDTO,
  type StatsPayload,
  type StatsResponse
} from '../api/shorturl'

const TrendChart = defineAsyncComponent(() => import('../components/TrendChart.vue'))

type DateRangeStr = [string, string]

const loading = ref(false)
const reportBusy = ref(false)
const exportPdfLoading = ref(false)
const stats = ref<StatsResponse | null>(null)

const reportJobId = ref<string | null>(null)
const jobStatus = ref('')
const aiReportData = ref<AIReportDTO | null>(null)
const editedMarkdownFromServer = ref('')
const editBuffer = ref('')
const activeReportTab = ref<'preview' | 'edit'>('preview')

const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)
/** 屏外白底排版，仅用于导出 PDF（与深色预览分离） */
const pdfSheetRef = ref<HTMLElement | null>(null)
const previewHtml = ref('')

const form = reactive<StatsPayload>({
  shortURL: '',
  startDate: '',
  endDate: ''
})

const dateRange = ref<DateRangeStr | null>(null)

const dateShortcuts = [
  {
    text: '最近 7 天',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setDate(end.getDate() - 6)
      return [start, end]
    }
  },
  {
    text: '最近 30 天',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setDate(end.getDate() - 29)
      return [start, end]
    }
  },
  {
    text: '本月',
    value: () => {
      const end = new Date()
      const start = new Date(end.getFullYear(), end.getMonth(), 1)
      return [start, end]
    }
  }
]

function formatDay(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

watch(
  dateRange,
  (v) => {
    if (v && v.length === 2 && v[0] && v[1]) {
      form.startDate = v[0]
      form.endDate = v[1]
    } else {
      form.startDate = ''
      form.endDate = ''
    }
  },
  { deep: true }
)

onMounted(() => {
  const end = new Date()
  const start = new Date()
  start.setDate(end.getDate() - 6)
  dateRange.value = [formatDay(start), formatDay(end)]
})

onBeforeUnmount(() => {
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = null
  }
})

const chartPoints = computed(() => {
  const arr = stats.value?.chartData ?? []
  return arr.map((x) => ({
    date: x.date,
    pv: Number(x.pv),
    uv: Number(x.uv)
  }))
})

const validateStatsForm = () => {
  if (!form.shortURL.trim()) {
    ElMessage.warning('请填写短链路径（surl）')
    return false
  }
  if (!form.startDate.trim() || !form.endDate.trim()) {
    ElMessage.warning('请选择统计日期范围')
    return false
  }
  if (form.startDate > form.endDate) {
    ElMessage.warning('开始日期不能晚于结束日期')
    return false
  }
  return true
}

const mobilePct = computed(() => {
  const r = stats.value?.deviceStats?.mobileRate
  if (r === undefined || r === null) return '—'
  return typeof r === 'number' ? r.toFixed(2) : String(r)
})

const deviceBreakdown = computed(() => stats.value?.deviceStats?.breakdown ?? [])
const geoByCountry = computed(() => stats.value?.geoByCountry ?? [])
const geoByRegion = computed(() => stats.value?.geoByRegion ?? [])

function buildFallbackMd(ar: AIReportDTO): string {
  const title = ar.title?.trim() || '短链访问分析报告'
  const lines = [`# ${title}`, '', '## 概述', '', ar.summary || '—', '', '## 趋势', '']
  for (const t of ar.trends ?? []) lines.push(`- ${t}`)
  lines.push('', '## 异常与风险', '')
  for (const a of ar.anomalies ?? []) lines.push(`- ${a}`)
  lines.push('', '## 建议', '')
  for (const s of ar.suggestions ?? []) lines.push(`- ${s}`)
  return lines.join('\n')
}

/** 展示用 Markdown：优先服务端保存的编辑稿，其次模型 markdown，最后由结构化字段拼装 */
const displayMarkdown = computed(() => {
  if (editedMarkdownFromServer.value.trim()) return editedMarkdownFromServer.value
  const ar = aiReportData.value
  if (!ar) return ''
  if (ar.markdown?.trim()) return ar.markdown.trim()
  return buildFallbackMd(ar)
})

watch(
  () => displayMarkdown.value,
  async (md) => {
    if (!md) {
      previewHtml.value = ''
      return
    }
    previewHtml.value = await marked.parse(md)
  },
  { immediate: true }
)

watch(activeReportTab, (tab) => {
  if (tab === 'edit') {
    editBuffer.value = displayMarkdown.value
  }
})

const queryStats = async () => {
  if (!validateStatsForm()) return
  loading.value = true
  try {
    const { data } = await getStats(form)
    stats.value = data
    ElMessage.success('统计查询成功')
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, '统计查询失败'))
  } finally {
    loading.value = false
  }
}

function stopPoll() {
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = null
  }
}

function startPoll(jobId: string) {
  stopPoll()
  reportBusy.value = true
  let attempts = 0
  pollTimer.value = setInterval(async () => {
    attempts++
    try {
      const { data } = await getAnalyzeReportStatus(jobId)
      jobStatus.value = data.status
      if (data.aiReport) aiReportData.value = data.aiReport
      editedMarkdownFromServer.value = data.markdownEdited ?? ''

      const done = data.status === 'completed' || data.status === 'failed' || attempts >= 90
      if (done) {
        stopPoll()
        reportBusy.value = false
        if (data.status === 'completed') ElMessage.success('AI 报告已生成')
        if (data.status === 'failed') ElMessage.warning(data.error || '报告生成失败（已尽可能降级）')
        if (attempts >= 90 && data.status !== 'completed' && data.status !== 'failed') {
          ElMessage.warning('等待超时，请稍后刷新页面或重新发起分析')
        }
      }
    } catch (e: unknown) {
      stopPoll()
      reportBusy.value = false
      ElMessage.error(apiErrorMessage(e, '获取报告状态失败'))
    }
  }, 2000)
}

const queryAnalyze = async () => {
  if (!validateStatsForm()) return
  loading.value = true
  reportJobId.value = null
  jobStatus.value = ''
  aiReportData.value = null
  editedMarkdownFromServer.value = ''
  stopPoll()
  try {
    const { data } = await analyzeStats(form)
    stats.value = data.statistics
    reportJobId.value = data.reportJob.jobId
    jobStatus.value = data.reportJob.status
    ElMessage.success('统计已返回，AI 报告后台生成中…')
    startPoll(data.reportJob.jobId)
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, '发起分析失败'))
  } finally {
    loading.value = false
  }
}

/** 将长截图按 A4 内边距纵向切片写入 PDF，避免「整页一张拉伸图」观感 */
function writeCanvasToA4Pdf(canvas: HTMLCanvasElement, fileName: string) {
  const marginMm = 12
  const pdf = new jsPDF({ unit: 'mm', format: 'a4', orientation: 'portrait' })
  const pageW = pdf.internal.pageSize.getWidth()
  const pageH = pdf.internal.pageSize.getHeight()
  const imgWMm = pageW - 2 * marginMm
  const imgHMm = (canvas.height * imgWMm) / canvas.width
  const innerHmm = pageH - 2 * marginMm

  let renderedMm = 0
  const eps = 0.05
  while (renderedMm + eps < imgHMm) {
    const sliceMm = Math.min(innerHmm, imgHMm - renderedMm)
    const topPx = (renderedMm / imgHMm) * canvas.height
    let slicePx = (sliceMm / imgHMm) * canvas.height
    const maxPx = canvas.height - topPx
    if (slicePx > maxPx) slicePx = maxPx

    const sliceCanvas = document.createElement('canvas')
    sliceCanvas.width = canvas.width
    sliceCanvas.height = Math.max(1, Math.ceil(slicePx))
    const ctx = sliceCanvas.getContext('2d')
    if (!ctx) break
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, sliceCanvas.width, sliceCanvas.height)
    ctx.drawImage(canvas, 0, topPx, canvas.width, slicePx, 0, 0, canvas.width, slicePx)

    const data = sliceCanvas.toDataURL('image/png')
    pdf.addImage(data, 'PNG', marginMm, marginMm, imgWMm, sliceMm)
    renderedMm += sliceMm
    if (renderedMm + eps < imgHMm) {
      pdf.addPage()
    }
  }
  pdf.save(fileName)
}

const saveEditedMarkdown = async () => {
  if (!reportJobId.value) {
    ElMessage.warning('暂无关联的报告任务')
    return
  }
  try {
    await updateAnalyzeReportMarkdown(reportJobId.value, editBuffer.value)
    editedMarkdownFromServer.value = editBuffer.value
    ElMessage.success('编辑内容已保存')
    activeReportTab.value = 'preview'
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, '保存失败'))
  }
}

const exportPdf = async () => {
  const el = pdfSheetRef.value
  if (!el || !displayMarkdown.value.trim()) {
    ElMessage.warning('暂无可导出的报告内容')
    return
  }
  exportPdfLoading.value = true
  try {
    activeReportTab.value = 'preview'
    await nextTick()
    await new Promise<void>((r) => setTimeout(r, 250))
    const canvas = await html2canvas(el, {
      scale: 2,
      useCORS: true,
      logging: false,
      backgroundColor: '#ffffff',
      removeContainer: false,
      windowWidth: el.scrollWidth,
      windowHeight: el.scrollHeight,
      onclone: (_clonedDoc, node) => {
        const n = node as HTMLElement
        n.style.background = '#ffffff'
        n.style.boxShadow = 'none'
      }
    })
    writeCanvasToA4Pdf(canvas, `shorturl-ai-report-${form.shortURL || 'report'}.pdf`)
    ElMessage.success('已导出 PDF')
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, '导出失败'))
  } finally {
    exportPdfLoading.value = false
  }
}
</script>

<template>
  <div>
    <el-card class="admin-panel-card">
      <template #header><span class="panel-title">访问统计与 AI 分析</span></template>
      <el-form label-position="top" class="filter-form">
        <el-row :gutter="16">
          <el-col :xs="24" :sm="10" :md="8">
            <el-form-item label="短链路径（surl）">
              <el-input v-model="form.shortURL" placeholder="如 abc123" clearable />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="14" :md="16">
            <el-form-item label="统计日期范围">
              <el-date-picker
                v-model="dateRange"
                type="daterange"
                unlink-panels
                range-separator="至"
                start-placeholder="开始日期"
                end-placeholder="结束日期"
                value-format="YYYY-MM-DD"
                :shortcuts="dateShortcuts"
                style="width: 100%; max-width: 520px"
              />
              <div class="range-hint">
                按<strong>自然日</strong>汇总（结束日当天 23:59:59 仍计入）；图表为「按日」曲线。
              </div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item>
          <el-button type="primary" :loading="loading" @click="queryStats">查询统计</el-button>
          <el-button :loading="loading" @click="queryAnalyze">生成 AI 报告（异步）</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <template v-if="stats">
      <el-row :gutter="16" class="stat-row" style="margin-top: 16px">
        <el-col :xs="24" :sm="8">
          <el-card class="stat-tile" shadow="never">
            <div class="stat-label">总 PV</div>
            <div class="stat-value">{{ stats.totalPV ?? 0 }}</div>
          </el-card>
        </el-col>
        <el-col :xs="24" :sm="8">
          <el-card class="stat-tile" shadow="never">
            <div class="stat-label">区间 UV（去重 IP）</div>
            <div class="stat-value">{{ stats.totalUV ?? 0 }}</div>
          </el-card>
        </el-col>
        <el-col :xs="24" :sm="8">
          <el-card class="stat-tile" shadow="never">
            <div class="stat-label">移动端占比</div>
            <div class="stat-value">{{ mobilePct }}%</div>
          </el-card>
        </el-col>
      </el-row>

      <el-row :gutter="16">
        <el-col :xs="24" :md="12">
          <el-card class="admin-panel-card" style="margin-top: 16px">
            <template #header><span class="panel-title">设备分布</span></template>
            <el-table :data="deviceBreakdown">
              <el-table-column prop="device" label="设备类型" />
              <el-table-column prop="count" label="访问次数" />
            </el-table>
          </el-card>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-card class="admin-panel-card" style="margin-top: 16px">
            <template #header><span class="panel-title">国家聚合 Top10</span></template>
            <el-table :data="geoByCountry">
              <el-table-column prop="name" label="国家/地区" />
              <el-table-column prop="count" label="访问次数" />
            </el-table>
          </el-card>
        </el-col>
      </el-row>

      <el-card v-if="geoByRegion.length" class="admin-panel-card" style="margin-top: 16px">
        <template #header><span class="panel-title">地区聚合 Top10</span></template>
        <el-table :data="geoByRegion">
          <el-table-column prop="name" label="地区" />
          <el-table-column prop="count" label="访问次数" />
        </el-table>
      </el-card>

      <el-card v-if="chartPoints.length" class="admin-panel-card" style="margin-top: 16px">
        <template #header><span class="panel-title">访问趋势</span></template>
        <TrendChart :chart-data="chartPoints" />
      </el-card>

      <el-card class="admin-panel-card" style="margin-top: 16px">
        <template #header><span class="panel-title">按日明细</span></template>
        <el-table :data="stats.chartData || []">
          <el-table-column prop="date" label="日期" />
          <el-table-column prop="pv" label="PV" />
          <el-table-column prop="uv" label="UV" />
        </el-table>
      </el-card>

      <el-card
        v-if="stats.geoStats && stats.geoStats.length"
        class="admin-panel-card"
        style="margin-top: 16px"
      >
        <template #header><span class="panel-title">地域 Top（日志中国家/城市）</span></template>
        <el-table :data="stats.geoStats">
          <el-table-column prop="country" label="国家/地区" />
          <el-table-column prop="city" label="城市" />
          <el-table-column prop="count" label="次数" />
        </el-table>
      </el-card>
    </template>

    <el-card v-if="reportJobId" class="admin-panel-card" style="margin-top: 16px">
      <template #header>
        <div class="report-header">
          <span class="panel-title">AI 报告（异步）</span>
          <div class="report-meta">
            <el-tag v-if="jobStatus" size="small" type="info">任务：{{ jobStatus }}</el-tag>
            <el-tag v-if="reportBusy" size="small" type="warning">生成中…</el-tag>
          </div>
        </div>
      </template>

      <p v-if="!displayMarkdown && reportBusy" class="muted">模型正在生成结构化报告与 Markdown，请稍候…</p>
      <p v-else-if="!displayMarkdown && !reportBusy" class="muted">暂无 Markdown 内容</p>

      <template v-else>
        <el-tabs v-model="activeReportTab" class="report-tabs">
          <el-tab-pane label="预览（Markdown 渲染）" name="preview">
            <div class="markdown-body report-export" v-html="previewHtml" />
          </el-tab-pane>
          <el-tab-pane label="编辑 Markdown" name="edit">
            <el-input
              v-model="editBuffer"
              type="textarea"
              :rows="18"
              placeholder="在此修改报告正文（保存后预览与导出均使用编辑稿）"
            />
            <div class="edit-actions">
              <el-button type="primary" @click="saveEditedMarkdown">保存编辑</el-button>
            </div>
          </el-tab-pane>
        </el-tabs>

        <!-- 屏外白底 A4 版式，供 PDF 使用（不在界面显示） -->
        <div class="pdf-sheet-host" aria-hidden="true">
          <div ref="pdfSheetRef" class="pdf-sheet">
            <p class="pdf-sheet-meta">
              ShortLink 访问分析报告 · {{ form.shortURL || '—' }} · {{ form.startDate }} ～
              {{ form.endDate }}
            </p>
            <div class="pdf-sheet-content" v-html="previewHtml" />
          </div>
        </div>

        <div class="report-actions">
          <el-button :loading="exportPdfLoading" type="success" plain @click="exportPdf">
            导出 PDF
          </el-button>
          <span class="muted tiny">导出为白底 A4 文档样式；多页按纸张高度自动分页。</span>
        </div>
      </template>
    </el-card>
  </div>
</template>

<style scoped>
.panel-title {
  font-weight: 600;
}
.report-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.report-meta {
  display: flex;
  gap: 8px;
  align-items: center;
}
.stat-row {
  margin-bottom: 0;
}
.stat-tile {
  margin-bottom: 16px;
}
.stat-label {
  font-size: 0.85rem;
  color: var(--app-text-muted);
  margin-bottom: 0.35rem;
}
.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  letter-spacing: -0.02em;
}
.filter-form :deep(.el-form-item) {
  margin-bottom: 12px;
}
.range-hint {
  margin-top: 8px;
  font-size: 0.8rem;
  color: var(--app-text-muted);
  line-height: 1.5;
  max-width: 640px;
}
.muted {
  color: var(--app-text-muted);
}
.tiny {
  font-size: 0.75rem;
}
.report-tabs {
  margin-top: 8px;
}
.report-export {
  padding: 12px 16px;
  border-radius: 8px;
  border: 1px solid var(--app-border);
  min-height: 120px;
}
.edit-actions {
  margin-top: 12px;
}
.report-actions {
  margin-top: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.markdown-body :deep(h1) {
  font-size: 1.35rem;
  margin: 0 0 0.75rem;
}
.markdown-body :deep(h2) {
  font-size: 1.1rem;
  margin: 1.25rem 0 0.5rem;
}
.markdown-body :deep(p),
.markdown-body :deep(li) {
  line-height: 1.65;
  color: var(--app-text);
}
.markdown-body :deep(ul) {
  padding-left: 1.25rem;
}
.markdown-body :deep(code) {
  background: rgba(255, 255, 255, 0.06);
  padding: 0.1rem 0.35rem;
  border-radius: 4px;
}
</style>

<!-- PDF 专用排版：白底正文，与深色控制台预览无关 -->
<style>
.pdf-sheet-host {
  position: fixed;
  left: -14000px;
  top: 0;
  width: 794px;
  z-index: -20;
  pointer-events: none;
}
.pdf-sheet {
  box-sizing: border-box;
  width: 794px;
  padding: 36px 44px 40px;
  background: #ffffff;
  color: #1f2937;
  font-family:
    'Segoe UI',
    'PingFang SC',
    'Hiragino Sans GB',
    'Microsoft YaHei',
    sans-serif;
  font-size: 12.5pt;
  line-height: 1.68;
  -webkit-font-smoothing: antialiased;
}
.pdf-sheet-meta {
  margin: 0 0 18px;
  padding-bottom: 10px;
  border-bottom: 1px solid #e5e7eb;
  font-size: 9.5pt;
  color: #6b7280;
}
.pdf-sheet-content h1 {
  font-size: 20pt;
  font-weight: 700;
  color: #111827;
  margin: 0 0 14px;
  padding-bottom: 8px;
  border-bottom: 2px solid #e5e7eb;
}
.pdf-sheet-content h2 {
  font-size: 13.5pt;
  font-weight: 600;
  color: #374151;
  margin: 22px 0 10px;
}
.pdf-sheet-content h3 {
  font-size: 12pt;
  font-weight: 600;
  color: #4b5563;
  margin: 16px 0 8px;
}
.pdf-sheet-content p {
  margin: 0 0 10px;
  color: #374151;
}
.pdf-sheet-content ul,
.pdf-sheet-content ol {
  margin: 8px 0 12px;
  padding-left: 22px;
}
.pdf-sheet-content li {
  margin: 4px 0;
  color: #374151;
}
.pdf-sheet-content code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 10.5pt;
  background: #f3f4f6;
  color: #1f2937;
  padding: 2px 6px;
  border-radius: 3px;
}
.pdf-sheet-content pre {
  margin: 12px 0;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 12px 14px;
  overflow-x: auto;
}
.pdf-sheet-content pre code {
  background: transparent;
  padding: 0;
}
.pdf-sheet-content a {
  color: #2563eb;
  text-decoration: none;
}
.pdf-sheet-content blockquote {
  margin: 12px 0;
  padding: 8px 14px;
  border-left: 4px solid #d1d5db;
  background: #f9fafb;
  color: #4b5563;
}
.pdf-sheet-content table {
  width: 100%;
  border-collapse: collapse;
  margin: 12px 0;
  font-size: 11pt;
}
.pdf-sheet-content th,
.pdf-sheet-content td {
  border: 1px solid #e5e7eb;
  padding: 6px 10px;
  text-align: left;
}
.pdf-sheet-content th {
  background: #f3f4f6;
  font-weight: 600;
}
</style>
