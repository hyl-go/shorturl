import { createRouter, createWebHistory } from 'vue-router'
import Convert from '../views/Convert.vue'
import LinkList from '../views/LinkList.vue'
import AdminLogin from '../views/AdminLogin.vue'
import Analytics from '../views/Analytics.vue'
import Performance from '../views/Performance.vue'
import {
  ADMIN_ROUTE_ANALYTICS,
  ADMIN_ROUTE_HOME,
  ADMIN_ROUTE_LINKS,
  ADMIN_ROUTE_LOGIN,
  ADMIN_ROUTE_PERFORMANCE,
  isAdminAuthed
} from '../config/admin'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Convert, meta: { role: 'guest' } },
    { path: ADMIN_ROUTE_LOGIN, component: AdminLogin, meta: { role: 'guest' } },
    { path: ADMIN_ROUTE_HOME, redirect: ADMIN_ROUTE_LINKS, meta: { requiresAdmin: true } },
    { path: ADMIN_ROUTE_LINKS, component: LinkList, meta: { requiresAdmin: true, title: '链接管理' } },
    { path: ADMIN_ROUTE_ANALYTICS, component: Analytics, meta: { requiresAdmin: true, title: '数据分析' } },
    {
      path: ADMIN_ROUTE_PERFORMANCE,
      component: Performance,
      meta: { requiresAdmin: true, title: '性能面板' }
    }
  ]
})

router.beforeEach((to) => {
  if (to.meta.requiresAdmin && !isAdminAuthed()) {
    return ADMIN_ROUTE_LOGIN
  }
  if (to.path === ADMIN_ROUTE_LOGIN && isAdminAuthed()) {
    return ADMIN_ROUTE_HOME
  }
  return true
})

export default router
