<template>
  <div class="login">
    <div class="card">
      <h1>LAN Chat</h1>
      <p class="subtitle">Input username to start</p>

      <el-input
        v-model="username"
        placeholder="Username"
        size="large"
        maxlength="32"
        @keyup.enter="doLogin"
      />
      <el-button type="primary" size="large" :loading="loading" @click="doLogin">Enter</el-button>

      <div class="info">
        <el-divider>Scan QR on Phone</el-divider>
        <div class="qr-wrap">
          <canvas ref="qrCanvas"></canvas>
        </div>
        <p class="qr-tip">Select your LAN IP below:</p>
        <div class="ip-list" v-if="serverUrls.length">
          <button
            v-for="item in serverUrls"
            :key="item.url"
            type="button"
            class="ip-item"
            :class="{ active: selectedUrl === item.url }"
            @click="selectedUrl = item.url"
          >
            {{ item.url }}
          </button>
        </div>
        <p v-else class="qr-tip">Unable to detect a LAN address. Start the app on your desktop first.</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import QRCode from 'qrcode'
import { GetAllIPs, Login as WailsLogin } from '../../wailsjs/go/main/App'
import { getHttpBase } from '../utils/network'

const router = useRouter()
const username = ref('')
const loading = ref(false)
const serverUrls = ref([])
const selectedUrl = ref('')
const qrCanvas = ref(null)

const dedupeUrls = (urls) => [...new Set((urls || []).filter(Boolean))]
const blockedQrHosts = new Set(['wails.localhost', 'localhost', '127.0.0.1'])

const currentOrigin = computed(() => {
  if (typeof window === 'undefined') return ''
  return window.location.origin
})

const isWailsRuntime = () => typeof window !== 'undefined' && !!window.go?.main?.App

const isReachableQrUrl = (value) => {
  if (!value) return false
  try {
    const url = new URL(value)
    return !blockedQrHosts.has(url.hostname)
  } catch {
    return false
  }
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

const toLanUrls = (ips) =>
  dedupeUrls((ips || []).map((ip) => `http://${ip}:5200`)).filter(isReachableQrUrl)

const loadLanUrlsFromWails = async () => {
  if (!isWailsRuntime()) return []
  try {
    const ips = await GetAllIPs()
    return toLanUrls(ips)
  } catch {
    return []
  }
}

const loadLanUrlsFromHttp = async () => {
  const attempts = 6
  for (let index = 0; index < attempts; index += 1) {
    try {
      const response = await fetch(`${getHttpBase()}/api/qrcode`)
      if (!response.ok) throw new Error('Failed to load LAN address')
      const data = await response.json()
      const urls = dedupeUrls([...(data.all_urls || []), data.url]).filter(isReachableQrUrl)
      if (urls.length) return urls
    } catch {}
    await sleep(500)
  }
  return []
}

const waitForLocalServer = async () => {
  const attempts = isWailsRuntime() ? 12 : 3
  for (let index = 0; index < attempts; index += 1) {
    try {
      const response = await fetch(`${getHttpBase()}/api/health`)
      if (response.ok) return true
    } catch {}
    await sleep(500)
  }
  return false
}

const genQR = async () => {
  await nextTick()
  if (!qrCanvas.value || !selectedUrl.value) return
  await QRCode.toCanvas(qrCanvas.value, selectedUrl.value, {
    width: 200,
    margin: 2,
    color: { dark: '#111827', light: '#ffffff' }
  })
}

watch(selectedUrl, () => {
  genQR().catch(() => {})
})

onMounted(async () => {
  const saved = localStorage.getItem('user')
  if (saved) {
    try {
      const parsed = JSON.parse(saved)
      if (parsed?.id || parsed?.user_id) {
        router.push('/chat')
        return
      }
    } catch {}
  }

  const urlsFromWails = await loadLanUrlsFromWails()
  const urlsFromHttp = urlsFromWails.length ? [] : await loadLanUrlsFromHttp()
  const fallback = dedupeUrls([currentOrigin.value, getHttpBase()]).filter(isReachableQrUrl)
  const urls = dedupeUrls([...urlsFromWails, ...urlsFromHttp, ...fallback])

  serverUrls.value = urls.map((url) => ({ url }))
  selectedUrl.value = serverUrls.value[0]?.url || ''

  genQR().catch(() => {})
})

const doLogin = async () => {
  const trimmed = username.value.trim()
  if (!trimmed) {
    ElMessage.warning('Enter username')
    return
  }

  loading.value = true
  try {
    if (isWailsRuntime()) {
      const result = await WailsLogin(trimmed)
      if (result?.error) {
        throw new Error(result.error)
      }
      localStorage.setItem('token', result.id || '')
      localStorage.setItem('user', JSON.stringify(result))
      ElMessage.success('Welcome')
      router.push('/chat')
      return
    }

    const ready = await waitForLocalServer()
    if (!ready) {
      throw new Error('Local service is still starting. Please wait 2-3 seconds and try again.')
    }

    const response = await fetch(`${getHttpBase()}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: trimmed,
        device_type: /mobile/i.test(navigator.userAgent) ? 'mobile' : 'web',
        device_name: navigator.userAgent
      })
    })
    const result = await response.json()
    if (!response.ok || result.error) {
      throw new Error(result.error || 'Login failed')
    }
    localStorage.setItem('token', result.token || '')
    localStorage.setItem('user', JSON.stringify(result.user || result))
    ElMessage.success('Welcome')
    router.push('/chat')
  } catch (error) {
    ElMessage.error(`Login failed: ${error.message}`)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login {
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background:
    radial-gradient(circle at top left, rgba(56, 189, 248, 0.28), transparent 32%),
    radial-gradient(circle at bottom right, rgba(14, 165, 233, 0.24), transparent 30%),
    linear-gradient(145deg, #0f172a 0%, #1e293b 45%, #0f766e 100%);
}

.card {
  width: min(100%, 440px);
  padding: 32px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.35);
  text-align: center;
}

h1 {
  margin-bottom: 6px;
  color: #0f172a;
  font-size: 30px;
}

.subtitle {
  margin-bottom: 24px;
  color: #64748b;
}

.card :deep(.el-input) {
  margin-bottom: 14px;
}

.card :deep(.el-button) {
  width: 100%;
  height: 44px;
  border-radius: 12px;
}

.info {
  margin-top: 24px;
}

.qr-wrap {
  display: flex;
  justify-content: center;
  margin: 16px 0 12px;
}

.qr-wrap canvas {
  width: 200px;
  max-width: 100%;
  height: auto;
  border-radius: 16px;
  background: #fff;
  padding: 10px;
  box-shadow: inset 0 0 0 1px #e2e8f0;
}

.qr-tip {
  margin-bottom: 12px;
  color: #64748b;
  font-size: 13px;
}

.ip-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ip-item {
  width: 100%;
  padding: 12px 14px;
  border: 1px solid #cbd5e1;
  border-radius: 12px;
  background: #f8fafc;
  color: #0f172a;
  text-align: left;
  cursor: pointer;
  font-family: Consolas, Monaco, monospace;
  word-break: break-all;
  transition: all 0.18s ease;
}

.ip-item:hover,
.ip-item.active {
  border-color: #0ea5e9;
  background: #e0f2fe;
  box-shadow: 0 10px 24px rgba(14, 165, 233, 0.16);
}

@media (max-width: 640px) {
  .login {
    padding: 14px;
    align-items: stretch;
  }

  .card {
    padding: 22px 18px;
    border-radius: 18px;
  }

  h1 {
    font-size: 26px;
  }
}
</style>
