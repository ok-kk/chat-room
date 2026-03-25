<template>
  <div class="chat">
    <div class="header">
      <div>
        <h2>LAN Chat</h2>
        <p>{{ username || 'Anonymous' }}<span v-if="onlineCount"> · {{ onlineCount }} online</span></p>
      </div>
      <div class="actions">
        <el-button size="small" @click="router.push('/files')">Files</el-button>
        <el-button size="small" :icon="RefreshRight" :loading="refreshing" @click="refreshMessages">Refresh</el-button>
        <el-tag :type="connected ? 'success' : 'danger'" size="small">{{ connected ? 'Online' : 'Offline' }}</el-tag>
        <el-button :icon="SwitchButton" circle size="small" @click="logout" />
      </div>
    </div>

    <div class="messages" ref="msgBox">
      <div v-if="!messages.length" class="empty">No messages yet</div>
      <div v-for="(msg, i) in messages" :key="msg.id || `${msg.created_at}-${i}`" class="msg" :class="{ self: msg.sender_id === userId }">
        <el-avatar :size="36" :style="{ background: color(msg.sender_name) }">{{ msg.sender_name?.[0] }}</el-avatar>
        <div class="body">
          <div class="meta">
            <span class="name">{{ msg.sender_name }}</span>
            <span class="time">{{ fmtTime(msg.created_at) }}</span>
            <button class="del-btn" type="button" @click="delMsg(msg, i)">&times;</button>
          </div>
          <div v-if="msg.msg_type === 'text'" class="content">{{ msg.content }}</div>
          <div v-else class="file-box">
            <img
              v-if="isImg(msg.file_name)"
              :src="makeAbsoluteUrl(`/api/thumbnail/${fileId(msg.file_url)}`)"
              class="thumb"
              @click="dl(msg.file_url)"
            />
            <div v-else class="file-icon">&#128196;</div>
            <div class="finfo">
              <div class="fname">{{ msg.file_name }}</div>
              <div class="fsize">{{ fmtSize(msg.file_size) }}</div>
            </div>
            <div class="file-btns">
              <button class="btn-dl" type="button" @click="dl(msg.file_url)">Download</button>
              <button class="btn-del" type="button" @click="delMsg(msg, i)">Delete</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="input-bar">
      <input ref="fileInput" type="file" multiple hidden @change="handleFiles" />
      <button class="btn-attach" type="button" @click="fileInput?.click()">&#128206;</button>
      <div v-if="uploading" class="upload-info">Uploading {{ upDone }}/{{ upTotal }}</div>
      <input
        v-model="input"
        class="text-input"
        placeholder="Type a message..."
        @keyup.enter="send"
        :disabled="!connected"
      />
      <button class="btn-send" type="button" @click="send" :disabled="!connected || !input.trim()">Send</button>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { RefreshRight, SwitchButton } from '@element-plus/icons-vue'
import { getHttpBase, getWsBase, makeAbsoluteUrl } from '../utils/network'
import { DeleteMessage as WailsDeleteMessage, GetMessages as WailsGetMessages } from '../../wailsjs/go/main/App'

const router = useRouter()
const userId = ref('')
const username = ref('')
const messages = ref([])
const input = ref('')
const connected = ref(false)
const msgBox = ref(null)
const fileInput = ref(null)
const uploading = ref(false)
const upDone = ref(0)
const upTotal = ref(0)
const onlineUsers = ref([])
const refreshing = ref(false)
let ws = null
let reconnectTimer = null
let isLeaving = false

const isWailsRuntime = () => typeof window !== 'undefined' && !!window.go?.main?.App

onMounted(() => {
  const saved = localStorage.getItem('user')
  if (!saved) {
    router.push('/')
    return
  }

  try {
    const user = JSON.parse(saved)
    userId.value = user.id || user.user_id
    username.value = user.username || 'Anonymous'
  } catch {
    localStorage.removeItem('user')
    router.push('/')
    return
  }

  loadHistory({ silent: true })
  connectWS()
})

onUnmounted(() => {
  isLeaving = true
  clearTimeout(reconnectTimer)
  if (ws) ws.close()
})

const connectWS = () => {
  clearTimeout(reconnectTimer)
  if (!userId.value || !username.value) return

  ws = new WebSocket(
    `${getWsBase()}/ws?user_id=${encodeURIComponent(userId.value)}&user_name=${encodeURIComponent(username.value)}&room_id=default`
  )

  ws.onopen = () => {
    connected.value = true
  }

  ws.onclose = () => {
    connected.value = false
    if (!isLeaving) {
      reconnectTimer = setTimeout(connectWS, 2000)
    }
  }

  ws.onerror = () => {
    connected.value = false
  }

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    if (data.type === 'user_list') {
      onlineUsers.value = Array.isArray(data.data) ? data.data : []
      return
    }

    if (data.type === 'message_deleted') {
      removeMessage(data.id, data.data || {})
      return
    }

    if (data.type !== 'chat') return

    messages.value.push({
      id: data.id || `${data.from}-${data.timestamp}-${Math.random()}`,
      sender_id: data.from,
      sender_name: data.from_name,
      content: data.content,
      msg_type: data.msg_type || 'text',
      file_url: data.file_url,
      file_name: data.file_name,
      file_size: data.file_size,
      created_at: data.timestamp
    })
    nextTick(scroll)
  }
}

const loadHistory = async (options = {}) => {
  const { silent = false } = options
  refreshing.value = true
  try {
    for (let attempt = 0; attempt < 3; attempt += 1) {
      try {
        if (isWailsRuntime()) {
          const data = await WailsGetMessages('default', 1)
          messages.value = Array.isArray(data) ? [...data].reverse() : []
        } else {
          const response = await fetch(`${getHttpBase()}/api/messages?room_id=default&page=1&page_size=100`)
          if (!response.ok) throw new Error(`History request failed with status ${response.status}`)
          const data = await response.json()
          messages.value = [...(data.messages || [])].reverse()
        }
        nextTick(scroll)
        if (!silent) {
          ElMessage.success('Messages refreshed')
        }
        return
      } catch (error) {
        if (attempt === 2) {
          ElMessage.error(error?.message || 'Failed to load history')
        }
        await new Promise((resolve) => setTimeout(resolve, 300))
      }
    }
  } finally {
    refreshing.value = false
  }
}

const refreshMessages = async () => {
  if (refreshing.value) return
  await loadHistory()
}

const send = () => {
  if (!input.value.trim() || !ws || ws.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({ type: 'chat', content: input.value.trim(), room_id: 'default', msg_type: 'text' }))
  input.value = ''
}

const handleFiles = async (event) => {
  const files = Array.from(event.target.files || [])
  if (!files.length) return

  uploading.value = true
  upTotal.value = files.length
  upDone.value = 0

  for (const file of files) {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('uploader_id', userId.value)
    formData.append('uploader_name', username.value)

    try {
      const response = await fetch(`${getHttpBase()}/api/upload`, { method: 'POST', body: formData })
      if (!response.ok) throw new Error('Upload failed')
      upDone.value += 1
    } catch {
      ElMessage.error(`Failed: ${file.name}`)
    }
  }

  uploading.value = false
  event.target.value = ''
  ElMessage.success(`Uploaded ${upDone.value} file(s)`)
}

const delMsg = async (msg, idx) => {
  if (!window.confirm('Delete this message?')) return

  try {
    if (isWailsRuntime()) {
      const result = await WailsDeleteMessage(msg.id || '', msg.sender_id || '', msg.content || '', msg.created_at || '', msg.file_url || '')
      if (result?.error) throw new Error(result.error)
    } else {
      const response = await fetch(`${getHttpBase()}/api/message/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: msg.id,
          sender_id: msg.sender_id,
          created_at: msg.created_at,
          content: msg.content,
          file_url: msg.file_url
        })
      })

      const result = await response.json()
      if (!response.ok || result.error) throw new Error(result.error || 'Delete failed')
    }

    messages.value.splice(idx, 1)
    ElMessage.success('Deleted')
  } catch (error) {
    ElMessage.error(error.message || 'Delete failed')
  }
}

const fileId = (url) => (url ? url.split('/').pop() : '')
const onlineCount = computed(() => onlineUsers.value.length)
const isImg = (name) => /\.(jpg|jpeg|png|gif|bmp|webp)$/i.test(name || '')
const dl = (url) => url && window.open(makeAbsoluteUrl(url), '_blank')
const scroll = () => {
  if (msgBox.value) {
    msgBox.value.scrollTop = msgBox.value.scrollHeight
  }
}

const removeMessage = (id, payload = {}) => {
  const index = messages.value.findIndex((item) => {
    if (id && item.id === id) return true
    return (
      item.sender_id === payload.sender_id &&
      item.content === payload.content &&
      item.created_at === payload.created_at
    )
  })
  if (index !== -1) {
    messages.value.splice(index, 1)
  }
}

const logout = () => {
  isLeaving = true
  clearTimeout(reconnectTimer)
  if (ws) ws.close()
  localStorage.removeItem('token')
  localStorage.removeItem('user')
  router.push('/')
}

const fmtTime = (value) => {
  if (!value) return ''
  return new Date(value).toLocaleString([], {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const fmtSize = (bytes) => {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let size = Number(bytes)
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size.toFixed(1)} ${units[index]}`
}

const color = (name = '') => {
  const palette = ['#0284c7', '#0f766e', '#0891b2', '#ea580c', '#7c3aed']
  let hash = 0
  for (let i = 0; i < name.length; i += 1) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return palette[Math.abs(hash) % palette.length]
}
</script>

<style scoped>
.chat {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #e2e8f0;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 18px;
  background: rgba(255, 255, 255, 0.94);
  border-bottom: 1px solid #cbd5e1;
}

.header h2 {
  margin: 0;
  color: #0f172a;
}

.header p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
}

.actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.messages {
  flex: 1;
  overflow-y: auto;
  padding: 18px;
}

.empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #64748b;
}

.msg {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}

.msg.self {
  flex-direction: row-reverse;
}

.body {
  max-width: min(70%, 720px);
}

.meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 12px;
}

.msg.self .meta {
  justify-content: flex-end;
}

.name {
  color: #334155;
  font-weight: 600;
}

.time {
  color: #94a3b8;
}

.del-btn {
  border: none;
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  font-size: 18px;
}

.content,
.file-box {
  border-radius: 16px;
  padding: 12px 14px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
  word-break: break-word;
}

.msg.self .content,
.msg.self .file-box {
  background: #0ea5e9;
  color: #fff;
}

.thumb {
  width: 100%;
  max-width: 220px;
  max-height: 180px;
  object-fit: cover;
  border-radius: 10px;
  display: block;
  margin-bottom: 10px;
  cursor: pointer;
}

.file-icon {
  font-size: 26px;
  margin-bottom: 8px;
}

.finfo {
  margin-bottom: 10px;
}

.fname {
  font-weight: 600;
}

.fsize {
  opacity: 0.82;
  font-size: 12px;
}

.file-btns {
  display: flex;
  gap: 8px;
}

.btn-dl,
.btn-del,
.btn-attach,
.btn-send {
  border: none;
  cursor: pointer;
}

.btn-dl,
.btn-del {
  padding: 6px 10px;
  border-radius: 999px;
}

.btn-dl {
  background: #0369a1;
  color: #fff;
}

.btn-del {
  background: rgba(239, 68, 68, 0.9);
  color: #fff;
}

.input-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 18px;
  background: rgba(255, 255, 255, 0.95);
  border-top: 1px solid #cbd5e1;
}

.btn-attach {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: #dbeafe;
}

.upload-info {
  color: #0369a1;
  font-size: 12px;
  white-space: nowrap;
}

.text-input {
  flex: 1;
  min-width: 0;
  padding: 10px 14px;
  border: 1px solid #cbd5e1;
  border-radius: 12px;
  outline: none;
}

.text-input:focus {
  border-color: #0ea5e9;
}

.btn-send {
  padding: 10px 18px;
  border-radius: 12px;
  background: #0ea5e9;
  color: #fff;
}

.btn-send:disabled {
  background: #94a3b8;
  cursor: not-allowed;
}

@media (max-width: 768px) {
  .header,
  .input-bar,
  .messages {
    padding-left: 12px;
    padding-right: 12px;
  }

  .body {
    max-width: 82%;
  }

  .actions {
    gap: 6px;
  }
}
</style>
