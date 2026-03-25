import { getHttpBase } from './network'

const buildUrl = (path, params) => {
  const url = new URL(path, `${getHttpBase()}/`)
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      url.searchParams.set(key, value)
    }
  })
  return url.toString()
}

const request = async (path, options = {}) => {
  const headers = new Headers(options.headers || {})
  const token = localStorage.getItem('token')
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(buildUrl(path, options.params), {
    method: options.method || 'GET',
    headers,
    body: options.body
  })

  const contentType = response.headers.get('content-type') || ''
  const data = contentType.includes('application/json') ? await response.json() : await response.text()

  if (!response.ok) {
    const message = typeof data === 'object' && data?.error ? data.error : `Request failed with status ${response.status}`
    throw new Error(message)
  }

  return { data, status: response.status }
}

const api = {
  get(path, options = {}) {
    return request(path, { ...options, method: 'GET' })
  },

  post(path, body, options = {}) {
    const isFormData = typeof FormData !== 'undefined' && body instanceof FormData
    const headers = { ...(options.headers || {}) }
    if (!isFormData && !headers['Content-Type']) {
      headers['Content-Type'] = 'application/json'
    }

    return request(path, {
      ...options,
      method: 'POST',
      headers,
      body: isFormData || typeof body === 'string' ? body : JSON.stringify(body)
    })
  }
}

export default api
