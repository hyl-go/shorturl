<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ADMIN_ROUTE_ANALYTICS,
  ADMIN_ROUTE_PERFORMANCE,
  ADMIN_ROUTE_LINKS,
  ADMIN_ROUTE_LOGIN,
  isAdminAuthed,
  setAdminAuthed
} from './config/admin'

const route = useRoute()
const router = useRouter()
const authed = computed(() => isAdminAuthed())

const isGuestConvert = computed(() => route.path === '/')
const isAdminLogin = computed(() => route.path === ADMIN_ROUTE_LOGIN)
const isAdminArea = computed(() => route.path.startsWith('/admin'))

const layout = computed(() => {
  if (isGuestConvert.value) return 'guest'
  if (isAdminLogin.value) return 'login'
  if (isAdminArea.value) return 'admin'
  return 'guest'
})

const logout = () => {
  setAdminAuthed(false)
  router.replace('/')
}
</script>

<template>
  <!-- 游客：仅短链转换，无侧栏、无数据入口 -->
  <div v-if="layout === 'guest'" class="guest-layout guest-page">
    <header class="guest-header">
      <div class="guest-brand">
        <span class="guest-logo">ShortLink</span>
        <span class="guest-tagline">快速生成短链接 · 可选 AI 增强</span>
      </div>
    </header>
    <main class="guest-main">
      <router-view />
    </main>
    <footer class="guest-footer">仅供访客生成短链使用 · 管理功能需单独入口</footer>
  </div>

  <!-- 管理员登录：独立全屏页，不暴露菜单 -->
  <div v-else-if="layout === 'login'" class="login-layout admin-login-page">
    <router-view />
  </div>

  <!-- 管理员：链接列表 / 数据分析 -->
  <el-container v-else class="admin-shell">
    <el-aside width="240px" class="admin-aside">
      <div class="admin-brand">
        <span class="admin-logo">ShortLink</span>
        <span class="admin-badge">管理后台</span>
      </div>
      <el-menu
        :default-active="$route.path"
        router
        background-color="transparent"
        text-color="#b8c5d6"
        active-text-color="#ffffff"
      >
        <el-menu-item :index="ADMIN_ROUTE_LINKS">
          <span>链接管理</span>
        </el-menu-item>
        <el-menu-item :index="ADMIN_ROUTE_ANALYTICS">
          <span>数据分析</span>
        </el-menu-item>
        <el-menu-item :index="ADMIN_ROUTE_PERFORMANCE">
          <span>性能面板</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container direction="vertical">
      <el-header class="admin-header">
        <span class="admin-title">{{ route.meta.title ?? '控制台' }}</span>
        <div class="admin-header-actions">
          <span v-if="authed" class="admin-user">已登录</span>
          <el-button type="primary" plain size="small" @click="logout">退出</el-button>
        </div>
      </el-header>
      <el-main class="admin-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.guest-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: radial-gradient(ellipse 120% 80% at 50% -20%, rgba(61, 139, 253, 0.25), transparent),
    linear-gradient(180deg, #0f1419 0%, #151c28 100%);
}

.guest-header {
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid var(--app-border);
  backdrop-filter: blur(8px);
}

.guest-brand {
  max-width: 720px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.guest-logo {
  font-size: 1.35rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  background: linear-gradient(135deg, #e8edf4 0%, #7eb8ff 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.guest-tagline {
  font-size: 0.875rem;
  color: var(--app-text-muted);
}

.guest-main {
  flex: 1;
  padding: 2rem 1.5rem 3rem;
  max-width: 720px;
  margin: 0 auto;
  width: 100%;
  box-sizing: border-box;
}

.guest-footer {
  text-align: center;
  padding: 1rem;
  font-size: 0.75rem;
  color: var(--app-text-muted);
  border-top: 1px solid var(--app-border);
}

.login-layout {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
  background: radial-gradient(ellipse 80% 60% at 50% 0%, rgba(61, 139, 253, 0.2), transparent),
    linear-gradient(180deg, #0f1419 0%, #151c28 100%);
}

.admin-shell {
  min-height: 100vh;
  background: #0f1419;
}

.admin-aside {
  background: linear-gradient(180deg, #151c28 0%, #0f1419 100%);
  border-right: 1px solid var(--app-border);
  padding: 1rem 0;
}

.admin-brand {
  padding: 0 1.25rem 1.25rem;
  border-bottom: 1px solid var(--app-border);
  margin-bottom: 0.5rem;
}

.admin-logo {
  display: block;
  font-weight: 700;
  font-size: 1.1rem;
  color: var(--app-text);
}

.admin-badge {
  display: inline-block;
  margin-top: 0.35rem;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--app-text-muted);
}

.admin-header {
  height: 56px !important;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 1.5rem;
  border-bottom: 1px solid var(--app-border);
  background: rgba(21, 28, 40, 0.85);
}

.admin-title {
  font-weight: 600;
  font-size: 1rem;
}

.admin-header-actions {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.admin-user {
  font-size: 0.85rem;
  color: var(--app-text-muted);
}

.admin-main {
  background: #121922;
  padding: 1.5rem;
  min-height: calc(100vh - 56px);
}

:deep(.el-menu-item.is-active) {
  background: var(--app-accent-soft) !important;
  border-radius: 8px;
  margin: 4px 8px;
  width: auto;
}

:deep(.el-menu-item) {
  border-radius: 8px;
  margin: 4px 8px;
  width: auto;
}
</style>
