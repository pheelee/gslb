import { createRouter, createWebHistory } from 'vue-router'
import Layout from '@/components/Layout.vue'
import Dashboard from '@/views/Dashboard.vue'
import ConfigList from '@/views/ConfigList.vue'
import ConfigForm from '@/views/ConfigForm.vue'
import ConfigDetail from '@/views/ConfigDetail.vue'
import AuditLog from '@/views/AuditLog.vue'
import AdminUsers from '@/views/AdminUsers.vue'
import { useAuthStore } from '@/stores/auth'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    requiresRole?: string[]
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: Layout,
      children: [
        {
          path: '',
          name: 'Dashboard',
          component: Dashboard,
          meta: { requiresAuth: true },
        },
        {
          path: '/configs',
          name: 'ConfigList',
          component: ConfigList,
          meta: { requiresAuth: true },
        },
        {
          path: '/configs/new',
          name: 'ConfigNew',
          component: ConfigForm,
          meta: { requiresAuth: true, requiresRole: ['admin', 'operator'] },
        },
        {
          path: '/configs/:id',
          name: 'ConfigDetail',
          component: ConfigDetail,
          meta: { requiresAuth: true },
        },
        {
          path: '/configs/:id/edit',
          name: 'ConfigEdit',
          component: ConfigForm,
          meta: { requiresAuth: true, requiresRole: ['admin', 'operator'] },
        },
        {
          path: '/audit-log',
          name: 'AuditLog',
          component: AuditLog,
          meta: { requiresAuth: true },
        },
        {
          path: '/admin/users',
          name: 'AdminUsers',
          component: AdminUsers,
          meta: { requiresAuth: true, requiresRole: ['admin'] },
        },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // Fetch user on first navigation (status starts as 'unknown')
  if (auth.status === 'unknown') {
    await auth.fetchUser()
  }

  // When OIDC is disabled, allow everything through
  if (!auth.oidcEnabled) return true

  // Route requires authentication
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    auth.login()
    return false
  }

  // Route requires specific role(s)
  if (to.meta.requiresRole && auth.isAuthenticated) {
    const hasRole = to.meta.requiresRole.some(r => auth.hasRole(r))
    if (!hasRole) return { name: 'Dashboard' }
  }

  return true
})

export default router
