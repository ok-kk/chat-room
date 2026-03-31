<template>
  <div class="chat" @dragover.prevent @drop.prevent="handleDomDrop">
    <div class="header">
      <div class="header-main">
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

    <div v-if="windowDropActive" class="window-drop-overlay">
      <div class="window-drop-card">
        <div class="window-drop-title">Drop files to send</div>
        <div class="window-drop-subtitle">Release anywhere in the window</div>
      </div>
    </div>

    <div class="messages" ref="msgBox">
      <div v-if="!messages.length" class="empty">No messages yet</div>
      <div v-for="msg in messages" :key="messageKey(msg)" class="msg" :class="{ self: msg.sender_id === userId }">
        <el-avatar :size="36" :style="{ background: color(msg.sender_name) }">{{ msg.sender_name?.[0] }}</el-avatar>
        <div class="body">
          <div class="meta">
            <span class="name">{{ msg.sender_name }}</span>
            <span class="time">{{ fmtTime(msg.created_at) }}</span>
            <button class="del-btn" type="button" @click="delMsg(msg)">&times;</button>
          </div>
          <div v-if="msg.msg_type === 'text'" class="content text-content">{{ msg.content }}</div>
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
              <button class="btn-del" type="button" @click="delMsg(msg)">Delete</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="input-bar">
      <input ref="fileInput" type="file" multiple hidden @change="handleFiles" />
      <button class="btn-attach" type="button" @click="fileInput?.click()">&#128206;</button>
      <div v-if="uploading" class="upload-info">Uploading {{ upDone }}/{{ upTotal }}</div>
      <textarea
        ref="textInput"
        v-model="input"
        class="text-input"
        placeholder="Type a message..."
        @keydown.enter.exact.prevent="send"
        @paste="handlePaste"
        @input="autoResizeInput"
        :disabled="!connected"
        rows="1"
      />
      <button class="btn-send" type="button" @click="send" :disabled="!connected || !input.trim()">Send</button>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RefreshRight, SwitchButton } from '@element-plus/icons-vue'
import { OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'
import { getHttpBase, getWsBase, makeAbsoluteUrl } from '../utils/network'
import { DeleteMessage as WailsDeleteMessage, GetMessages as WailsGetMessages, SaveUploadedFile } from '../../wailsjs/go/main/App'

const router = useRouter()
const userId = ref('')
const username = ref('')
const messages = ref([])
const input = ref('')
const connected = ref(false)
const msgBox = ref(null)
const fileInput = ref(null)
const textInput = ref(null)
const uploading = ref(false)
const upDone = ref(0)
const upTotal = ref(0)
const onlineUsers = ref([])
const refreshing = ref(false)
const windowDropActive = ref(false)
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
  setupWindowDrop()
})

onUnmounted(() => {
  isLeaving = true
  clearTimeout(reconnectTimer)
  if (ws) ws.close()
  if (isWailsRuntime()) {
    OnFileDropOff()
  }
  window.removeEventListener('dragenter', handleWindowDragEnter)
  window.removeEventListener('dragleave', handleWindowDragLeave)
  window.removeEventListener('drop', handleWindowDropEnd)
})

const setupWindowDrop = () => {
  if (isWailsRuntime()) {
    OnFileDrop(async (_x, _y, paths) => {
      if (!Array.isArray(paths) || !paths.length) return
      await confirmAndUploadPaths(paths)
    }, false)
  }

  window.addEventListener('dragenter', handleWindowDragEnter)
  window.addEventListener('dragleave', handleWindowDragLeave)
  window.addEventListener('drop', handleWindowDropEnd)
}

const handleWindowDragEnter = (event) => {
  if (event.dataTransfer?.types?.includes('Files')) {
    windowDropActive.value = true
  }
}

const handleWindowDragLeave = (event) => {
  if (event.relatedTarget == null) {
    windowDropActive.value = false
  }
}

const handleWindowDropEnd = () => {
  windowDropActive.value = false
}

const handleDomDrop = async (event) => {
  windowDropActive.value = false
  const files = Array.from(event.dataTransfer?.files || [])
  if (!files.length) return

  try {
    await ElMessageBox.confirm(`Send ${files.length} dropped file(s)?`, 'Send Files', {
      confirmButtonText: 'Send',
      cancelButtonText: 'Cancel',
      type: 'info'
    })
  } catch {
    return
  }

  await uploadFromBrowserFiles(files)
}

const confirmAndUploadPaths = async (paths) => {
  windowDropActive.value = false

  try {
    await ElMessageBox.confirm(`Send ${paths.length} dropped file(s)?`, 'Send Files', {
      confirmButtonText: 'Send',
      cancelButtonText: 'Cancel',
      type: 'info'
    })
  } catch {
    return
  }

  await uploadFromPaths(paths)
}

const connectWS = () => {
  clearTimeout(reconnectTimer)
  if (!userId.value || !username.value) return

  ws = new WebSocket(`${getWsBase()}/ws?user_id=${encodeURIComponent(userId.value)}&user_name=${encodeURIComponent(username.value)}&room_id=default`)

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
      removeMessageById(data.id)
      return
    }

    if (data.type !== 'chat') return

    const incoming = normalizeMessage({
      id: data.id,
      sender_id: data.from,
      sender_name: data.from_name,
      content: data.content,
      msg_type: data.msg_type || 'text',
      file_url: data.file_url,
      file_name: data.file_name,
      file_size: data.file_size,
      created_at: data.timestamp
    })

    if (!messages.value.some((item) => item.id && item.id === incoming.id)) {
      messages.value.push(incoming)
      nextTick(scroll)
    }
  }
}

const refreshMessages = () => loadHistory({ silent: false })

const loadHistory = async ({ silent = false } = {}) => {
  refreshing.value = !silent
  try {
    let loaded = []
    if (isWailsRuntime()) {
      loaded = await WailsGetMessages('default', 1)
    } else {
      const response = await fetch(`${getHttpBase()}/api/messages?room_id=default&page=1&page_size=100`)
      if (!response.ok) throw new Error(`History request failed with status ${response.status}`)
      const data = await response.json()
      loaded = data.messages || []
    }

    messages.value = [...loaded].reverse().map(normalizeMessage)
    nextTick(() => {
      scroll()
      autoResizeInput()
    })
  } catch (error) {
    if (!silent) {
      ElMessage.error(error?.message || 'Failed to load history')
    }
  } finally {
    refreshing.value = false
  }
}

const send = () => {
  if (!input.value.trim() || !ws || ws.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({ type: 'chat', content: input.value, room_id: 'default', msg_type: 'text' }))
  input.value = ''
  autoResizeInput()
}

const handlePaste = async (event) => {
  const clipboard = event.clipboardData
  if (!clipboard) return

  const files = Array.from(clipboard.items || [])
    .filter((item) => item.kind === 'file')
    .map((item) => item.getAsFile())
    .filter(Boolean)

  if (!files.length) return
  event.preventDefault()

  try {
    await ElMessageBox.confirm(`Send ${files.length} pasted file(s)?`, 'Send Files', {
      confirmButtonText: 'Send',
      cancelButtonText: 'Cancel',
      type: 'info'
    })
  } catch {
    return
  }

  await uploadFromBrowserFiles(files)
}

const handleFiles = async (event) => {
  const files = Array.from(event.target.files || [])
  if (!files.length) return
  await uploadFromBrowserFiles(files)
  event.target.value = ''
}

const withUploadState = async (items, worker) => {
  uploading.value = true
  upTotal.value = items.length
  upDone.value = 0

  try {
    for (const item of items) {
      await worker(item)
    }
    ElMessage.success(`Uploaded ${upDone.value} file(s)`)
  } finally {
    uploading.value = false
  }
}

const uploadFromBrowserFiles = async (files) => {
  await withUploadState(files, async (file) => {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('uploader_id', userId.value)
    formData.append('uploader_name', username.value)

    try {
      const response = await fetch(`${getHttpBase()}/api/upload`, { method: 'POST', body: formData })
      if (!response.ok) {
        const detail = await response.text().catch(() => '')
        throw new Error(detail || `Upload failed with status ${response.status}`)
      }
      upDone.value += 1
    } catch (error) {
      ElMessage.error(`${file.name}: ${error.message || 'Upload failed'}`)
    }
  })
}

const uploadFromPaths = async (paths) => {
  await withUploadState(paths, async (path) => {
    const fileName = path.split('\\').pop() || path
    try {
      const result = await SaveUploadedFile(path, userId.value, username.value)
      if (result?.error) {
        throw new Error(result.error)
      }
      upDone.value += 1
    } catch (error) {
      ElMessage.error(`${fileName}: ${error.message || 'Upload failed'}`)
    }
  })
}

const delMsg = async (msg) => {
  if (!msg?.id) {
    ElMessage.error('Message ID missing. Refresh and try again.')
    return
  }
  if (!window.confirm('Delete this message?')) return

  try {
    if (isWailsRuntime()) {
      const result = await WailsDeleteMessage(msg.id, msg.sender_id || '', msg.content || '', msg.created_at || '', msg.file_url || '')
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

    removeMessageById(msg.id)
    ElMessage.success('Deleted')
  } catch (error) {
    ElMessage.error(error.message || 'Delete failed')
  }
}

const normalizeMessage = (msg) => ({
  ...msg,
  id: msg?.id || '',
  content: msg?.content ?? '',
  sender_id: msg?.sender_id ?? '',
  sender_name: msg?.sender_name ?? 'Unknown',
  msg_type: msg?.msg_type || 'text',
  file_url: msg?.file_url || '',
  file_name: msg?.file_name || '',
  file_size: msg?.file_size || 0,
  created_at: msg?.created_at || ''
})

const messageKey = (msg) => msg.id || `${msg.sender_id}|${msg.created_at}|${msg.file_url || msg.content}`

const removeMessageById = (id) => {
  if (!id) return
  messages.value = messages.value.filter((item) => item.id !== id)
}

const autoResizeInput = () => {
  if (!textInput.value) return

  const lineCount = (input.value.match(/\n/g) || []).length + 1
  textInput.value.style.height = '44px'
  textInput.value.style.overflowY = 'hidden'

  if (lineCount <= 3) {
    const nextHeight = Math.min(textInput.value.scrollHeight, 88)
    textInput.value.style.height = `${Math.max(nextHeight, 44)}px`
    return
  }

  textInput.value.style.height = '88px'
  textInput.value.style.overflowY = 'auto'
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

.header-main {
  min-width: 0;
  flex: 1;
}

.header h2 {
  margin: 0;
  color: #0f172a;
  line-height: 1.2;
}

.header p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
  white-space: normal;
  word-break: break-word;
}

.window-drop-overlay {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.22);
  backdrop-filter: blur(2px);
  z-index: 30;
}

.window-drop-card {
  padding: 28px 32px;
  border: 2px dashed #0284c7;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.96);
  text-align: center;
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.18);
}

.window-drop-title {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
}

.window-drop-subtitle {
  margin-top: 8px;
  color: #64748b;
}

.actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
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

.text-content {
  white-space: pre-wrap;
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
  align-items: flex-end;
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
  height: 44px;
  min-height: 44px;
  max-height: 88px;
  padding: 10px 14px;
  border: 1px solid #cbd5e1;
  border-radius: 12px;
  outline: none;
  resize: none;
  overflow-y: hidden;
  line-height: 1.5;
  font: inherit;
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

  .header {
    align-items: flex-start;
    flex-direction: column;
  }

  .actions {
    width: 100%;
    gap: 6px;
    justify-content: flex-start;
  }

  .header h2 {
    font-size: 22px;
  }

  .header p {
    max-width: 100%;
  }
}
</style>
