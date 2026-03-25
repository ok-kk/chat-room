<template>
  <div class="files-container">
    <div class="files-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" circle @click="goBack" />
        <div>
          <h2>File Transfer</h2>
          <p>Share files across devices on the same LAN</p>
        </div>
      </div>
      <div class="header-actions">
        <input ref="fileInput" type="file" multiple hidden @change="handleSelectedFiles" />
        <el-button type="primary" :icon="Upload" :loading="uploading" @click="fileInput?.click()">Upload Files</el-button>
      </div>
    </div>

    <div
      class="upload-area"
      :class="{ dragging: isDragging }"
      @dragover.prevent="isDragging = true"
      @dragleave.prevent="isDragging = false"
      @drop.prevent="handleDrop"
    >
      <div class="upload-content">
        <el-icon size="48" color="#0ea5e9"><Upload /></el-icon>
        <p>Drop files here to upload</p>
        <p class="upload-hint">
          Supports multiple files, up to 100 MB per file
          <span v-if="uploading"> · {{ uploadedCount }}/{{ uploadTotal }} uploaded</span>
        </p>
      </div>
    </div>

    <div class="files-list">
      <el-table :data="files" style="width: 100%" v-loading="loading">
        <el-table-column prop="original_name" label="File" min-width="220">
          <template #default="{ row }">
            <div class="file-name-cell">
              <el-icon size="18" color="#0ea5e9"><Document /></el-icon>
              <span>{{ row.original_name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="file_size" label="Size" width="120">
          <template #default="{ row }">{{ formatFileSize(row.file_size) }}</template>
        </el-table-column>
        <el-table-column prop="uploader_name" label="Uploader" width="140" />
        <el-table-column prop="created_at" label="Time" width="180">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="Actions" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="downloadFile(row)">Download</el-button>
            <el-button type="success" link @click="copyLink(row)">Copy Link</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50]"
          :total="total"
          layout="total, sizes, prev, pager, next"
          @size-change="handlePageSizeChange"
          @current-change="loadFiles"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Document, Upload } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { ClipboardSetText, CanResolveFilePaths, ResolveFilePaths } from '../../wailsjs/runtime/runtime'
import { GetFiles as WailsGetFiles, SaveUploadedFile } from '../../wailsjs/go/main/App'
import { useUserStore } from '../stores/user'
import api from '../utils/api'
import { makeAbsoluteUrl } from '../utils/network'

const router = useRouter()
const userStore = useUserStore()

const files = ref([])
const loading = ref(false)
const uploading = ref(false)
const uploadedCount = ref(0)
const uploadTotal = ref(0)
const isDragging = ref(false)
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const fileInput = ref(null)

const isWailsRuntime = () => typeof window !== 'undefined' && !!window.go?.main?.App

onMounted(() => {
  userStore.loadUser()
  if (!userStore.isLoggedIn) {
    router.push('/')
    return
  }
  loadFiles()
})

const loadFiles = async () => {
  loading.value = true
  try {
    for (let attempt = 0; attempt < 3; attempt += 1) {
      try {
        if (isWailsRuntime()) {
          const data = await WailsGetFiles(currentPage.value)
          files.value = Array.isArray(data) ? data : []
          total.value = files.value.length
        } else {
          const response = await api.get('/api/files', {
            params: {
              page: currentPage.value,
              page_size: pageSize.value
            }
          })
          files.value = response.data.files || []
          total.value = response.data.total || 0
        }
        return
      } catch (error) {
        if (attempt === 2) {
          ElMessage.error(error?.message || 'Failed to load files')
        }
        await new Promise((resolve) => setTimeout(resolve, 300))
      }
    }
  } finally {
    loading.value = false
  }
}

const formatFileSize = (bytes) => {
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

const formatTime = (value) => {
  if (!value) return ''
  return new Date(value).toLocaleString('zh-CN')
}

const downloadFile = (file) => {
  window.open(makeAbsoluteUrl(`/api/download/${file.id}`), '_blank')
}

const copyLink = async (file) => {
  const link = makeAbsoluteUrl(`/api/download/${file.id}`)
  try {
    if (isWailsRuntime()) {
      await ClipboardSetText(link)
    } else if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(link)
    } else {
      const input = document.createElement('input')
      input.value = link
      document.body.appendChild(input)
      input.select()
      document.execCommand('copy')
      document.body.removeChild(input)
    }
    ElMessage.success('Link copied')
  } catch {
    ElMessage.error('Copy failed')
  }
}

const beforeUpload = (file) => {
  if (file.size > 100 * 1024 * 1024) {
    ElMessage.error(`${file.name}: file size cannot exceed 100 MB`)
    return false
  }
  return true
}

const uploadFiles = async (selectedFiles) => {
  const validFiles = selectedFiles.filter(beforeUpload)
  if (!validFiles.length) return

  uploading.value = true
  uploadTotal.value = validFiles.length
  uploadedCount.value = 0

  try {
    if (isWailsRuntime() && (await CanResolveFilePaths())) {
      const resolvedPaths = await ResolveFilePaths(validFiles)
      for (let index = 0; index < validFiles.length; index += 1) {
        const filePath = resolvedPaths[index]
        const file = validFiles[index]
        if (!filePath) {
          ElMessage.error(`${file.name} upload failed`)
          continue
        }
        const result = await SaveUploadedFile(filePath, userStore.user?.id || '', userStore.user?.username || '')
        if (result?.error) {
          ElMessage.error(`${file.name} upload failed: ${result.error}`)
          continue
        }
        uploadedCount.value += 1
      }
    } else {
      for (const file of validFiles) {
        const formData = new FormData()
        formData.append('file', file)
        formData.append('uploader_id', userStore.user?.id || '')
        formData.append('uploader_name', userStore.user?.username || '')

        try {
          await api.post('/api/upload', formData)
          uploadedCount.value += 1
        } catch (error) {
          ElMessage.error(`${file.name} upload failed: ${error.message || 'unknown error'}`)
        }
      }
    }

    await loadFiles()
    ElMessage.success(`Uploaded ${uploadedCount.value} file(s)`)
  } finally {
    uploading.value = false
  }
}

const handleSelectedFiles = async (event) => {
  const selectedFiles = Array.from(event.target.files || [])
  await uploadFiles(selectedFiles)
  event.target.value = ''
}

const handleDrop = async (event) => {
  isDragging.value = false
  const droppedFiles = Array.from(event.dataTransfer?.files || [])
  await uploadFiles(droppedFiles)
}

const handlePageSizeChange = () => {
  currentPage.value = 1
  loadFiles()
}

const goBack = () => {
  router.push('/chat')
}
</script>

<style scoped>
.files-container {
  min-height: 100%;
  display: flex;
  flex-direction: column;
  background:
    linear-gradient(180deg, rgba(224, 242, 254, 0.65), rgba(248, 250, 252, 0.9)),
    #f8fafc;
}

.files-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 18px 22px;
  background: rgba(255, 255, 255, 0.94);
  border-bottom: 1px solid #dbeafe;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-actions {
  display: flex;
  align-items: center;
}

.header-left h2 {
  margin: 0;
  color: #0f172a;
}

.header-left p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
}

.upload-area {
  margin: 20px 22px 0;
  padding: 42px 20px;
  border: 2px dashed #7dd3fc;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.85);
  text-align: center;
  transition: all 0.2s ease;
}

.upload-area.dragging,
.upload-area:hover {
  border-color: #0284c7;
  background: #e0f2fe;
  transform: translateY(-1px);
}

.upload-content {
  color: #475569;
}

.upload-content p {
  margin-top: 12px;
}

.upload-hint {
  font-size: 12px;
  color: #64748b;
}

.files-list {
  flex: 1;
  margin: 20px 22px 22px;
  padding: 18px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.95);
  overflow: auto;
}

.file-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

@media (max-width: 768px) {
  .files-header,
  .upload-area,
  .files-list {
    margin-left: 12px;
    margin-right: 12px;
  }

  .files-header {
    padding: 14px 12px;
    flex-wrap: wrap;
  }

  .upload-area {
    padding: 30px 14px;
  }

  .files-list {
    margin-top: 12px;
    margin-bottom: 12px;
    padding: 12px;
  }
}
</style>
