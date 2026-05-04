<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { apiErrorMessage, convertShortUrl, openShortUrlDisplay, type ConvertPayload } from '../api/shorturl'
import AINameSuggestions from '../components/AINameSuggestions.vue'
import SafetyBadge from '../components/SafetyBadge.vue'
import CategoryTag from '../components/CategoryTag.vue'

type ExpireMode = 'never' | 'preset' | 'custom'

const loading = ref(false)
const result = ref<{
  shortURL: string
  expireAt?: string
  category?: string
  safetyStatus?: string
  aiSuggestions?: string[]
  /** same_active | renewed_expired | reactivated_deleted | inserted_new */
  linkReuse?: string
} | null>(null)

function reuseHint(key?: string): string {
  switch (key) {
    case 'same_active':
      return '当前为已有有效短链，已直接返回'
    case 'renewed_expired':
      return '该长链曾绑定短链但已过期，已按本次设置在原有短码上续约。'
    case 'reactivated_deleted':
      return '该长链记录曾被删除，已在原短码上复活并应用本次设置。'
    case 'inserted_new':
      return '已为新长链生成短码。'
    default:
      return ''
  }
}

const expireMode = ref<ExpireMode>('never')
const expirePreset = ref<'30m' | '1h' | '1d' | '7d'>('1h')
const customAmount = ref(30)
const customUnit = ref<'minute' | 'hour' | 'day' | 'week' | 'month' | 'year'>('minute')

const form = reactive({
  longURL: '',
  customShortURL: '',
  enableAI: true
})

function buildExpirePart(): Pick<
  ConvertPayload,
  'expirePreset' | 'expireAfterValue' | 'expireAfterUnit' | 'expireAt'
> {
  if (expireMode.value === 'never') return {}
  if (expireMode.value === 'preset') return { expirePreset: expirePreset.value }
  const v = Number(customAmount.value)
  if (!Number.isFinite(v) || v <= 0) {
    throw new Error('自定义过期请输入大于 0 的数值')
  }
  return { expireAfterValue: Math.floor(v), expireAfterUnit: customUnit.value }
}

const onSubmit = async () => {
  if (!form.longURL) {
    ElMessage.warning('请输入长链接')
    return
  }
  let expirePart: ReturnType<typeof buildExpirePart>
  try {
    expirePart = buildExpirePart()
  } catch (e: unknown) {
    ElMessage.warning(e instanceof Error ? e.message : '过期设置无效')
    return
  }
  loading.value = true
  try {
    const payload: ConvertPayload = {
      longURL: form.longURL,
      customShortURL: form.customShortURL.trim(),
      enableAI: form.enableAI,
      ...expirePart
    }
    const { data } = await convertShortUrl(payload)
    result.value = data
    ElMessage.success('短链生成成功')
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, '生成失败'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="convert-page">
    <div class="convert-hero">
      <h1 class="convert-title">把长链接变短</h1>
      <p class="convert-desc">输入原始 URL，一键生成短链。开启 AI 后可获得分类、安全与命名建议（可选）。</p>
    </div>

    <el-card class="convert-card" shadow="never">
      <template #header>
        <span class="card-head">创建短链</span>
      </template>
      <el-form label-position="top">
        <el-form-item label="长链接">
          <el-input
            v-model="form.longURL"
            size="large"
            clearable
            placeholder="https://example.com/very/long/path"
          />
        </el-form-item>
        <el-form-item label="自定义短链（可选）">
          <el-input v-model="form.customShortURL" clearable placeholder="如 my-link，仅字母数字与连字符" />
        </el-form-item>

        <el-form-item label="过期时间">
          <el-radio-group v-model="expireMode" class="expire-mode">
            <el-radio-button label="never">永不过期</el-radio-button>
            <el-radio-button label="preset">快捷选项</el-radio-button>
            <el-radio-button label="custom">自定义时长</el-radio-button>
          </el-radio-group>

          <div v-if="expireMode === 'preset'" class="expire-block">
            <el-radio-group v-model="expirePreset" size="small">
              <el-radio-button label="30m">半小时</el-radio-button>
              <el-radio-button label="1h">一小时</el-radio-button>
              <el-radio-button label="1d">一天</el-radio-button>
              <el-radio-button label="7d">一周</el-radio-button>
            </el-radio-group>
          </div>

          <div v-else-if="expireMode === 'custom'" class="expire-block expire-custom">
            <el-input-number v-model="customAmount" :min="1" :max="999999" controls-position="right" />
            <el-select v-model="customUnit" style="width: 140px">
              <el-option label="分钟" value="minute" />
              <el-option label="小时" value="hour" />
              <el-option label="天" value="day" />
              <el-option label="周" value="week" />
              <el-option label="月（自然月）" value="month" />
              <el-option label="年（自然年）" value="year" />
            </el-select>
            <span class="expire-hint">从生成时刻起计算</span>
          </div>
        </el-form-item>

        <el-form-item label="启用 AI 分析">
          <el-switch v-model="form.enableAI" active-text="开" inactive-text="关" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" :loading="loading" class="submit-btn" @click="onSubmit">
            生成短链
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card v-if="result" class="result-card" shadow="never">
      <template #header>
        <span class="card-head">生成结果</span>
      </template>
    <div class="result-short">{{ result.shortURL }}</div>
    <p v-if="reuseHint(result.linkReuse)" class="reuse-line">{{ reuseHint(result.linkReuse) }}</p>
    <el-button type="primary" plain size="small" class="try-open-btn" @click="openShortUrlDisplay(result.shortURL)">
        试跳短链（新标签打开）
      </el-button>
      <div v-if="result.expireAt" class="result-row">
        <span class="result-label">过期</span>
        <span class="result-value">{{ result.expireAt }}</span>
      </div>
      <div class="result-row">
        <span class="result-label">分类</span>
        <CategoryTag :category="result.category" />
      </div>
      <div class="result-row">
        <span class="result-label">安全</span>
        <SafetyBadge :status="result.safetyStatus" />
      </div>
      <div class="result-row result-ai">
        <span class="result-label">AI 建议</span>
        <AINameSuggestions :suggestions="result.aiSuggestions || []" />
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.convert-hero {
  margin-bottom: 1.75rem;
}
.convert-title {
  margin: 0 0 0.5rem;
  font-size: 1.75rem;
  font-weight: 700;
  letter-spacing: -0.03em;
}
.convert-desc {
  margin: 0;
  font-size: 0.95rem;
  color: var(--app-text-muted);
  line-height: 1.55;
}
.convert-card,
.result-card {
  margin-bottom: 1.25rem;
}
.card-head {
  font-weight: 600;
}
.submit-btn {
  width: 100%;
  border-radius: 10px;
}
.expire-mode {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.expire-block {
  margin-top: 0.75rem;
}
.expire-custom {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.65rem;
}
.expire-hint {
  font-size: 0.8rem;
  color: var(--app-text-muted);
}
.reuse-line {
  margin: 0 0 0.75rem;
  font-size: 0.82rem;
  color: var(--app-text-muted);
  line-height: 1.45;
}
.result-short {
  font-size: 1rem;
  word-break: break-all;
  padding: 0.75rem 1rem;
  border-radius: 10px;
  background: var(--app-accent-soft);
  border: 1px solid var(--app-border);
  margin-bottom: 0.75rem;
  font-family: ui-monospace, monospace;
}
.try-open-btn {
  margin-bottom: 1rem;
}
.result-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.65rem;
}
.result-row.result-ai {
  align-items: flex-start;
}
.result-label {
  flex-shrink: 0;
  width: 4rem;
  font-size: 0.85rem;
  color: var(--app-text-muted);
}
.result-value {
  font-size: 0.9rem;
}
</style>
