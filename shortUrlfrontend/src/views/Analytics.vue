<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { analyzeStats, apiErrorMessage, getStats } from '../api/shorturl'
import type { StatsPayload } from '../api/shorturl'
import TrendChart from '../components/TrendChart.vue'

/** value-format 下为 YYYY-MM-DD 字符串元组 */
type DateRangeStr = [string, string]

const loading = ref(false)
const stats = ref<{
  totalPV?: number
  totalUV?: number
  chartData?: { date: string; pv: number; uv: number }[]
  deviceStats?: { mobileRate?: number }
  geoStats?: { country: string; city: string; count: number }[]
} | null>(null)
const report = ref<{
  aiReport?: {
    summary?: string
    trends?: string[]
    anomalies?: string[]
    suggestions?: string[]
  }
} | null>(null)

const form = reactive<StatsPayload>({
  shortURL: '',
  startDate: '',
  endDate: ''
})

/** 与后端约定一致：按自然日边界统计（结束日含整天）。暂不扩展到时分秒，否则需改 Stats API 与按日图表聚合逻辑 */
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

const queryAnalyze = async () => {
  if (!validateStatsForm()) return
  loading.value = true
  try {
    const { data } = await analyzeStats(form)
    report.value = data
    stats.value = data.statistics
    ElMessage.success('AI 报告生成成功')
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, 'AI 报告生成失败'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
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
              按<strong>自然日</strong>汇总（结束日当天 23:59:59 仍计入）；图表为「按日」曲线。若要做<strong>按小时</strong>区间，需要后端扩展时间段参数与聚合维度。
            </div>
          </el-form-item>
        </el-col>
      </el-row>
      <el-form-item>
        <el-button type="primary" :loading="loading" @click="queryStats">查询统计</el-button>
        <el-button :loading="loading" @click="queryAnalyze">生成 AI 报告</el-button>
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

  <el-card v-if="report" class="admin-panel-card" style="margin-top: 16px">
    <template #header><span class="panel-title">AI 报告</span></template>
    <p><b>概述：</b>{{ report.aiReport?.summary }}</p>
    <p><b>趋势：</b>{{ (report.aiReport?.trends || []).join('；') }}</p>
    <p><b>异常：</b>{{ (report.aiReport?.anomalies || []).join('；') || '无' }}</p>
    <p><b>建议：</b>{{ (report.aiReport?.suggestions || []).join('；') }}</p>
  </el-card>
</template>

<style scoped>
.panel-title {
  font-weight: 600;
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
</style>
