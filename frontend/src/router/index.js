import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', name: 'Login', component: () => import('../views/Login.vue') },
  { path: '/chat', name: 'Chat', component: () => import('../views/Chat.vue') },
  { path: '/files', name: 'Files', component: () => import('../views/Files.vue') },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
