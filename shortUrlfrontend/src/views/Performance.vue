<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { apiErrorMessage, getPerformanceSnapshot, type PerformanceSnapshot } from '../api/shorturl'

const loading = ref(false)
const data = ref<PerformanceSnapshot | null>(null)
const autoRefresh = ref(true)
let timer: ReturnType<typeof setInterval> | null = null

function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

async function load() {
  loading.value = true
  try {
    const { data: d } = await getPerformanceSnapshot()
    data.value = d
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, '加载性能数据失败'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await load()
  timer = setInterval(() => {
    if (autoRefresh.value) void load()
  }, 5000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="perf-page">
    <div class="perf-toolbar">
      <el-switch v-model="autoRefresh" active-text="每 5 秒刷新" />
      <el-button type="primary" :loading="loading" @click="load">立即刷新</el-button>
    </div>
    <p v-if="data?.collectedAt" class="perf-meta">采集时间：{{ data.collectedAt }}（CPU 采样约 200ms）</p>

    <el-row :gutter="16" v-loading="loading && !data">
      <el-col :xs="24" :md="12">
        <el-card shadow="never" class="perf-card">
          <template #header>主机</template>
          <el-descriptions v-if="data" :column="1" size="small" border>
            <el-descriptions-item label="主机名">{{ data.host.hostname || '—' }}</el-descriptions-item>
            <el-descriptions-item label="系统">{{ data.host.os }} / {{ data.host.platform }}</el-descriptions-item>
            <el-descriptions-item label="内核">{{ data.host.kernel || '—' }}</el-descriptions-item>
            <el-descriptions-item label="运行时间">{{ Math.floor(data.host.uptimeSec / 3600) }} 小时</el-descriptions-item>
            <el-descriptions-item label="进程数">{{ data.host.procs }}</el-descriptions-item>
            <el-descriptions-item label="Go 协程">{{ data.host.goRoutines }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-card shadow="never" class="perf-card">
          <template #header>CPU / 负载</template>
          <div v-if="data" class="perf-gauges">
            <div>
              <div class="label">CPU 使用率（瞬时）</div>
              <el-progress :percentage="Math.min(100, Math.round(data.cpu.usagePercent))" />
            </div>
            <el-descriptions :column="1" size="small" border class="mt">
              <el-descriptions-item label="Load1">
                {{ typeof data.cpu.load1 === 'number' ? data.cpu.load1.toFixed(2) : '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Load5">
                {{ typeof data.cpu.load5 === 'number' ? data.cpu.load5.toFixed(2) : '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="Load15">
                {{ typeof data.cpu.load15 === 'number' ? data.cpu.load15.toFixed(2) : '—' }}
              </el-descriptions-item>
            </el-descriptions>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mt-row">
      <el-col :xs="24" :md="12">
        <el-card shadow="never" class="perf-card">
          <template #header>内存</template>
          <div v-if="data">
            <el-progress :percentage="Math.min(100, Math.round(data.memory.usedPercent))" />
            <el-descriptions :column="1" size="small" border class="mt">
              <el-descriptions-item label="已用">{{ formatBytes(data.memory.usedBytes) }}</el-descriptions-item>
              <el-descriptions-item label="可用">{{ formatBytes(data.memory.availableBytes) }}</el-descriptions-item>
              <el-descriptions-item label="总计">{{ formatBytes(data.memory.totalBytes) }}</el-descriptions-item>
            </el-descriptions>
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-card shadow="never" class="perf-card">
          <template #header>磁盘（根分区 / Windows 为 C:）</template>
          <div v-if="data && data.disk.path">
            <el-progress :percentage="Math.min(100, Math.round(data.disk.usedPercent))" />
            <el-descriptions :column="1" size="small" border class="mt">
              <el-descriptions-item label="挂载">{{ data.disk.path }}</el-descriptions-item>
              <el-descriptions-item label="已用">{{ formatBytes(data.disk.usedBytes) }}</el-descriptions-item>
              <el-descriptions-item label="空闲">{{ formatBytes(data.disk.freeBytes) }}</el-descriptions-item>
              <el-descriptions-item label="总计">{{ formatBytes(data.disk.totalBytes) }}</el-descriptions-item>
            </el-descriptions>
          </div>
          <p v-else class="muted">磁盘信息不可用</p>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="perf-card mt-row">
      <template #header>磁盘 IO（累计）</template>
      <p v-if="data?.diskIO.note" class="muted small">{{ data.diskIO.note }}</p>
      <el-descriptions v-if="data" :column="2" size="small" border>
        <el-descriptions-item label="读字节">{{ formatBytes(Number(data.diskIO.readBytes)) }}</el-descriptions-item>
        <el-descriptions-item label="写字节">{{ formatBytes(Number(data.diskIO.writeBytes)) }}</el-descriptions-item>
        <el-descriptions-item label="读次数">{{ data.diskIO.readCount }}</el-descriptions-item>
        <el-descriptions-item label="写次数">{{ data.diskIO.writeCount }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-row :gutter="16" class="mt-row">
      <el-col :xs="24" :md="12">
        <el-card shadow="never" class="perf-card">
          <template #header>MySQL</template>
          <el-alert v-if="data && !data.mysql.ok" type="error" :title="data.mysql.error || '不可用'" show-icon class="mb" />
          <el-descriptions v-if="data && data.mysql.ok" :column="1" size="small" border>
            <el-descriptions-item label="延迟">{{ data.mysql.pingMs.toFixed(1) }} ms</el-descriptions-item>
            <el-descriptions-item label="版本">{{ data.mysql.version }}</el-descriptions-item>
            <el-descriptions-item label="max_connections">{{ data.mysql.maxConnections }}</el-descriptions-item>
            <el-descriptions-item label="Threads_connected">{{ data.mysql.threadsConnected }}</el-descriptions-item>
            <el-descriptions-item label="Threads_running">{{ data.mysql.threadsRunning }}</el-descriptions-item>
            <el-descriptions-item label="Questions">{{ data.mysql.questions }}</el-descriptions-item>
            <el-descriptions-item label="Slow_queries">{{ data.mysql.slowQueries }}</el-descriptions-item>
            <el-descriptions-item label="Uptime(s)">{{ data.mysql.uptimeSec }}</el-descriptions-item>
            <el-descriptions-item label="Max_used_connections">{{ data.mysql.maxUsedConnections }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-card shadow="never" class="perf-card">
          <template #header>Redis（CacheRedis 首节点）</template>
          <el-alert v-if="data && !data.redis.ok" type="error" :title="data.redis.error || '不可用'" show-icon class="mb" />
          <el-descriptions v-if="data && data.redis.ok" :column="1" size="small" border>
            <el-descriptions-item label="延迟">{{ data.redis.pingMs.toFixed(1) }} ms</el-descriptions-item>
            <el-descriptions-item label="版本">{{ data.redis.redisVersion }}</el-descriptions-item>
            <el-descriptions-item label="内存">{{ data.redis.usedMemoryHuman || formatBytes(data.redis.usedMemory) }}</el-descriptions-item>
            <el-descriptions-item label="connected_clients">{{ data.redis.connectedClients }}</el-descriptions-item>
            <el-descriptions-item label="instantaneous_ops_per_sec">{{ data.redis.instantaneousOpsPerSec }}</el-descriptions-item>
            <el-descriptions-item label="total_commands_processed">{{ data.redis.totalCommandsProcessed }}</el-descriptions-item>
            <el-descriptions-item label="keyspace_hits / misses">
              {{ data.redis.keyspaceHits }} / {{ data.redis.keyspaceMisses }}
            </el-descriptions-item>
            <el-descriptions-item label="aof_enabled">{{ data.redis.aofEnabled || '—' }}</el-descriptions-item>
            <el-descriptions-item label="rdb_last_save_time">{{ data.redis.rdbLastSaveTime || '—' }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.perf-page {
  max-width: 1100px;
}
.perf-toolbar {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.75rem;
}
.perf-meta {
  font-size: 0.8rem;
  color: var(--app-text-muted);
  margin-bottom: 1rem;
}
.perf-card {
  margin-bottom: 0;
  background: rgba(21, 28, 40, 0.6);
  border: 1px solid var(--app-border);
}
.mt-row {
  margin-top: 1rem;
}
.mt {
  margin-top: 0.75rem;
}
.mb {
  margin-bottom: 0.75rem;
}
.muted {
  color: var(--app-text-muted);
  font-size: 0.85rem;
}
.small {
  font-size: 0.75rem;
  margin-bottom: 0.5rem;
}
.perf-gauges .label {
  font-size: 0.8rem;
  color: var(--app-text-muted);
  margin-bottom: 0.35rem;
}
</style>
