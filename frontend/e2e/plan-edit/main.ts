import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import AdminPaymentPlansView from '../../src/views/admin/orders/AdminPaymentPlansView.vue'
import i18n, { loadLocaleMessages } from '../../src/i18n'
import '../../src/style.css'

async function bootstrap() {
  await Promise.all(['core', 'admin-settings', 'admin-resources', 'user-misc'].map(scope => loadLocaleMessages('en', scope as Parameters<typeof loadLocaleMessages>[1])))
  i18n.global.locale.value = 'en'
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: AdminPaymentPlansView }] })
  await router.push('/')
  createApp(AdminPaymentPlansView).use(createPinia()).use(router).use(i18n).mount('#app')
}
void bootstrap()
