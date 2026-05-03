<script setup lang="ts">
import { reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { ADMIN_CREDENTIALS, ADMIN_ROUTE_HOME, setAdminAuthed } from '../config/admin'

const router = useRouter()
const form = reactive({
  username: '',
  password: ''
})

const onLogin = () => {
  if (form.username === ADMIN_CREDENTIALS.username && form.password === ADMIN_CREDENTIALS.password) {
    setAdminAuthed(true)
    ElMessage.success('登录成功')
    router.replace(ADMIN_ROUTE_HOME)
    return
  }
  ElMessage.error('账号或密码错误')
}
</script>

<template>
  <div class="login-card-wrap">
    <div class="login-brand">
      <span class="login-logo">ShortLink</span>
      <p class="login-sub">管理员登录 · 请使用配置文件中的账号</p>
    </div>
    <el-card class="login-card" shadow="never">
      <el-form label-position="top" @submit.prevent="onLogin">
        <el-form-item label="账号">
          <el-input v-model="form.username" size="large" clearable autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            size="large"
            type="password"
            show-password
            autocomplete="current-password"
            @keyup.enter="onLogin"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" class="login-btn" native-type="submit">登录</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.login-card-wrap {
  width: 100%;
  max-width: 400px;
}
.login-brand {
  text-align: center;
  margin-bottom: 1.5rem;
}
.login-logo {
  font-size: 1.5rem;
  font-weight: 700;
  background: linear-gradient(135deg, #e8edf4 0%, #7eb8ff 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.login-sub {
  margin: 0.5rem 0 0;
  font-size: 0.85rem;
  color: var(--app-text-muted);
}
.login-card {
  border-radius: var(--app-radius);
}
.login-btn {
  width: 100%;
  border-radius: 10px;
}
</style>
