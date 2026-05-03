<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  apiErrorMessage,
  deleteLink,
  listLinkCategories,
  listLinks,
  openShortUrlDisplay,
  updateLink,
  type LinkListRow,
  type LinkUpdatePayload
} from '../api/shorturl'
import SafetyBadge from '../components/SafetyBadge.vue'
import CategoryTag from '../components/CategoryTag.vue'

/** 来自后端 `/links/categories`，与表里 AI 等写入的真实分类一致 */
const categorySelectOptions = ref<{ label: string; value: string }[]>([{ label: '全部', value: '' }])

const loading = ref(false)
const saving = ref(false)
const categoryFilter = ref('')
const total = ref(0)
const rows = ref<LinkListRow[]>([])

const page = ref(1)
const pageSize = ref(20)

const drawerVisible = ref(false)
const drawerRow = ref<LinkListRow | null>(null)

const editVisible = ref(false)
const editingId = ref(0)

type ExpireEditMode = 'keep' | 'never' | 'preset' | 'custom'

const expireEditMode = ref<ExpireEditMode>('keep')
const editExpirePreset = ref<'30m' | '1h' | '1d' | '7d'>('1h')
const editCustomAmount = ref(30)
const editCustomUnit = ref<'minute' | 'hour' | 'day' | 'week' | 'month' | 'year'>('minute')

const editForm = reactive({
  longURL: '',
  category: ''
})

const loadCategories = async () => {
  try {
    const { data } = await listLinkCategories()
    const cats = data.categories ?? []
    categorySelectOptions.value = [
      { label: '全部', value: '' },
      ...cats.map((c) => ({ label: c, value: c }))
    ]
  } catch (e: unknown) {
    ElMessage.warning(apiErrorMessage(e, '加载分类列表失败'))
    categorySelectOptions.value = [{ label: '全部', value: '' }]
  }
}

const load = async () => {
  loading.value = true
  try {
    const { data } = await listLinks({
      page: page.value,
      pageSize: pageSize.value,
      category: categoryFilter.value || undefined
    })
    total.value = data.total
    rows.value = data.list ?? []
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, '加载列表失败'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadCategories()
  await load()
})

watch([page, pageSize], () => {
  load()
})

watch(categoryFilter, () => {
  if (page.value !== 1) page.value = 1
  else load()
})

function openDetail(row: LinkListRow) {
  drawerRow.value = row
  drawerVisible.value = true
}

function openEdit(row: LinkListRow) {
  editingId.value = row.id
  editForm.longURL = row.longURL
  editForm.category = row.category ?? '其他'
  expireEditMode.value = 'keep'
  editExpirePreset.value = '1h'
  editCustomAmount.value = 30
  editCustomUnit.value = 'minute'
  editVisible.value = true
}

function buildEditPayload(): LinkUpdatePayload {
  const body: LinkUpdatePayload = {
    longURL: editForm.longURL.trim(),
    category: editForm.category.trim() || '其他'
  }
  switch (expireEditMode.value) {
    case 'keep':
      break
    case 'never':
      body.noExpire = true
      break
    case 'preset':
      body.expirePreset = editExpirePreset.value
      break
    case 'custom': {
      const v = Number(editCustomAmount.value)
      if (!Number.isFinite(v) || v <= 0) {
        throw new Error('自定义过期请输入大于 0 的数值')
      }
      body.expireAfterValue = Math.floor(v)
      body.expireAfterUnit = editCustomUnit.value
      break
    }
  }
  return body
}

async function submitEdit() {
  let payload: LinkUpdatePayload
  try {
    payload = buildEditPayload()
  } catch (e: unknown) {
    ElMessage.warning(e instanceof Error ? e.message : '过期设置无效')
    return
  }
  saving.value = true
  try {
    await updateLink(editingId.value, payload)
    ElMessage.success('已保存')
    editVisible.value = false
    await Promise.all([loadCategories(), load()])
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, '保存失败'))
  } finally {
    saving.value = false
  }
}

async function onDelete(row: LinkListRow) {
  try {
    await ElMessageBox.confirm(`确定删除短链「${row.shortPath}」吗？删除后跳转将失效（软删除）。`, '确认删除', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  saving.value = true
  try {
    await deleteLink(row.id)
    ElMessage.success('已删除')
    if (drawerVisible.value && drawerRow.value?.id === row.id) drawerVisible.value = false
    await Promise.all([loadCategories(), load()])
  } catch (e: unknown) {
    ElMessage.error(apiErrorMessage(e, '删除失败'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-card class="admin-panel-card">
    <template #header>
      <div class="panel-head">
        <span class="panel-title">链接列表</span>
        <el-select
          v-model="categoryFilter"
          placeholder="按分类筛选（来自数据库）"
          clearable
          filterable
          allow-create
          default-first-option
          style="width: min(100%, 320px)"
        >
          <el-option v-for="o in categorySelectOptions" :key="o.value || '__all'" :label="o.label" :value="o.value" />
        </el-select>
      </div>
    </template>
    <el-table v-loading="loading" :data="rows">
      <el-table-column prop="shortURL" label="短链" min-width="200">
        <template #default="{ row }">
          <span class="mono">{{ row.shortURL }}</span>
          <el-button link type="primary" size="small" @click="openShortUrlDisplay(row.shortURL)">打开</el-button>
        </template>
      </el-table-column>
      <el-table-column label="长链" min-width="260">
        <template #default="{ row }">
          <el-tooltip :content="row.longURL" placement="top">
            <span class="ellipsis">{{ row.longURL }}</span>
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column label="分类" width="110">
        <template #default="{ row }">
          <CategoryTag :category="row.category" />
        </template>
      </el-table-column>
      <el-table-column label="安全" width="100">
        <template #default="{ row }">
          <SafetyBadge :status="row.safetyStatus" />
        </template>
      </el-table-column>
      <el-table-column prop="expireAt" label="过期" width="168" />
      <el-table-column prop="createAt" label="创建" width="168" />
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="openDetail(row)">详情</el-button>
          <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" size="small" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <div style="margin-top: 16px; display: flex; justify-content: flex-end">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        background
      />
    </div>
  </el-card>

  <el-drawer v-model="drawerVisible" title="链接详情" size="440px" destroy-on-close>
    <template v-if="drawerRow">
      <dl class="detail-dl">
        <dt>短链</dt>
        <dd class="mono">{{ drawerRow.shortURL }}</dd>
        <dt>路径（surl）</dt>
        <dd class="mono">{{ drawerRow.shortPath }}</dd>
        <dt>长链</dt>
        <dd class="break">{{ drawerRow.longURL }}</dd>
        <dt>MD5（长链）</dt>
        <dd class="mono muted">{{ drawerRow.md5 || '—' }}</dd>
        <dt>分类</dt>
        <dd><CategoryTag :category="drawerRow.category" /></dd>
        <dt>安全</dt>
        <dd><SafetyBadge :status="drawerRow.safetyStatus" /></dd>
        <dt>过期时间</dt>
        <dd>{{ drawerRow.expireAt || '永不过期' }}</dd>
        <dt>创建 / 更新</dt>
        <dd>{{ drawerRow.createAt }} / {{ drawerRow.updateAt || '—' }}</dd>
        <dt>页面标题</dt>
        <dd>{{ drawerRow.pageTitle || '—' }}</dd>
        <dt>页面描述</dt>
        <dd class="break muted">{{ drawerRow.pageDescription || '—' }}</dd>
        <dt>AI 命名建议</dt>
        <dd>
          <template v-if="drawerRow.aiSuggestions?.length">
            <el-tag v-for="(s, i) in drawerRow.aiSuggestions" :key="i" size="small" class="tag-gap">{{ s }}</el-tag>
          </template>
          <span v-else class="muted">—</span>
        </dd>
      </dl>
    </template>
  </el-drawer>

  <el-dialog v-model="editVisible" title="编辑链接" width="560px" destroy-on-close @closed="editingId = 0">
    <el-form label-position="top">
      <el-form-item label="长链接">
        <el-input v-model="editForm.longURL" type="textarea" :rows="2" placeholder="修改后将重新校验可达性并重算 MD5" />
      </el-form-item>
      <el-form-item label="分类">
        <el-input v-model="editForm.category" placeholder="可与筛选下拉一致，任意文案" clearable />
      </el-form-item>
      <el-form-item label="过期策略">
        <el-radio-group v-model="expireEditMode" class="expire-radio">
          <el-radio-button label="keep">保持不变</el-radio-button>
          <el-radio-button label="never">改为永不过期</el-radio-button>
          <el-radio-button label="preset">快捷时长</el-radio-button>
          <el-radio-button label="custom">自定义</el-radio-button>
        </el-radio-group>
        <div v-if="expireEditMode === 'preset'" class="expire-sub">
          <el-radio-group v-model="editExpirePreset" size="small">
            <el-radio-button label="30m">半小时</el-radio-button>
            <el-radio-button label="1h">一小时</el-radio-button>
            <el-radio-button label="1d">一天</el-radio-button>
            <el-radio-button label="7d">一周</el-radio-button>
          </el-radio-group>
        </div>
        <div v-else-if="expireEditMode === 'custom'" class="expire-sub expire-custom">
          <el-input-number v-model="editCustomAmount" :min="1" :max="999999" controls-position="right" />
          <el-select v-model="editCustomUnit" style="width: 140px">
            <el-option label="分钟" value="minute" />
            <el-option label="小时" value="hour" />
            <el-option label="天" value="day" />
            <el-option label="周" value="week" />
            <el-option label="月" value="month" />
            <el-option label="年" value="year" />
          </el-select>
          <span class="hint">自保存时刻起算</span>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="editVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="submitEdit">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}
.panel-title {
  font-weight: 600;
}
.mono {
  font-family: ui-monospace, monospace;
  font-size: 0.85rem;
}
.ellipsis {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: bottom;
}
.break {
  word-break: break-all;
  line-height: 1.45;
}
.muted {
  color: var(--app-text-muted);
  font-size: 0.9rem;
}
.detail-dl {
  margin: 0;
}
.detail-dl dt {
  font-size: 0.75rem;
  color: var(--app-text-muted);
  margin-top: 0.85rem;
}
.detail-dl dt:first-child {
  margin-top: 0;
}
.detail-dl dd {
  margin: 0.25rem 0 0;
}
.tag-gap {
  margin-right: 0.35rem;
  margin-bottom: 0.35rem;
}
.expire-radio {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.expire-sub {
  margin-top: 0.65rem;
}
.expire-custom {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.hint {
  font-size: 0.8rem;
  color: var(--app-text-muted);
}
</style>
