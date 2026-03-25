import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '../utils/api'
import { useUserStore } from './user'

export const useChatStore = defineStore('chat', () => {
  const messages = ref([])
  const users = ref([])
  const socket = ref(null)
  const isConnected = ref(false)

  function connect(roomId = 'default') {
    const userStore = useUserStore()
    if (!userStore.user) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const wsUrl = `${protocol}//${host}/ws?user_id=${userStore.user.id}&user_name=${encodeURIComponent(userStore.user.username)}&room_id=${roomId}`

    socket.value = new WebSocket(wsUrl)

    socket.value.onopen = () => {
      isConnected.value = true
      console.log('WebSocket connected')
    }

    socket.value.onmessage = (event) => {
      const data = JSON.parse(event.data)
      handleMessage(data)
    }

    socket.value.onclose = () => {
      isConnected.value = false
      console.log('WebSocket disconnected')
      setTimeout(() => connect(roomId), 3000)
    }

    socket.value.onerror = (error) => {
      console.error('WebSocket error:', error)
    }
  }

  function handleMessage(data) {
    switch (data.type) {
      case 'chat':
        messages.value.push({
          id: Date.now(),
          sender_id: data.from,
          sender_name: data.from_name,
          content: data.content,
          msg_type: data.msg_type || 'text',
          file_url: data.file_url,
          file_name: data.file_name,
          file_size: data.file_size,
          created_at: data.timestamp
        })
        break
      case 'user_list':
        users.value = data.data
        break
      default:
        break
    }
  }

  function sendMessage(content, msgType = 'text') {
    if (!socket.value || socket.value.readyState !== WebSocket.OPEN) return

    const message = {
      type: 'chat',
      content,
      msg_type: msgType,
      room_id: 'default'
    }

    socket.value.send(JSON.stringify(message))
  }

  async function loadHistory(page = 1, pageSize = 50) {
    try {
      const response = await api.get('/api/messages', {
        params: { room_id: 'default', page, page_size: pageSize }
      })

      if (page === 1) {
        messages.value = response.data.messages.reverse()
      } else {
        messages.value = [...response.data.messages.reverse(), ...messages.value]
      }

      return response.data
    } catch (error) {
      console.error('Failed to load message history:', error)
    }
  }

  function disconnect() {
    if (socket.value) {
      socket.value.close()
      socket.value = null
    }
  }

  return { messages, users, isConnected, connect, sendMessage, loadHistory, disconnect }
})
