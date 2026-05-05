export const ADMIN_ROUTE_LOGIN = '/entry/admin-portal-9x7k'
export const ADMIN_ROUTE_HOME = '/admin'
export const ADMIN_ROUTE_LINKS = '/admin/links'
export const ADMIN_ROUTE_ANALYTICS = '/admin/analytics'
export const ADMIN_ROUTE_PERFORMANCE = '/admin/performance'

export const ADMIN_CREDENTIALS = {
  username: 'admin',
  password: 'ShortUrl@2026'
}

const ADMIN_AUTH_KEY = 'shorturl_admin_authed'

export const setAdminAuthed = (authed: boolean) => {
  if (authed) {
    localStorage.setItem(ADMIN_AUTH_KEY, '1')
    return
  }
  localStorage.removeItem(ADMIN_AUTH_KEY)
}

export const isAdminAuthed = () => localStorage.getItem(ADMIN_AUTH_KEY) === '1'
