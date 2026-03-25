const DEFAULT_PORT = '5200'
const WAILS_HOST = 'wails.localhost'

const hasBrowserLocation = () => typeof window !== 'undefined' && typeof window.location !== 'undefined'

export const getHttpBase = () => {
  if (!hasBrowserLocation()) {
    return `http://127.0.0.1:${DEFAULT_PORT}`
  }

  const { protocol, origin, hostname } = window.location
  if (hostname === WAILS_HOST) {
    return `http://127.0.0.1:${DEFAULT_PORT}`
  }

  if (protocol === 'http:' || protocol === 'https:') {
    return origin
  }

  return `http://127.0.0.1:${DEFAULT_PORT}`
}

export const getWsBase = () => {
  if (!hasBrowserLocation()) {
    return `ws://127.0.0.1:${DEFAULT_PORT}`
  }

  const { protocol, hostname } = window.location
  const wsProtocol = protocol === 'https:' ? 'wss:' : 'ws:'
  if (hostname === WAILS_HOST) {
    return `ws://127.0.0.1:${DEFAULT_PORT}`
  }

  if (protocol === 'http:' || protocol === 'https:') {
    return `${wsProtocol}//${window.location.host}`
  }

  return `ws://127.0.0.1:${DEFAULT_PORT}`
}

export const makeAbsoluteUrl = (path = '') => {
  if (!path) return getHttpBase()
  if (/^https?:\/\//i.test(path)) return path
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${getHttpBase()}${normalizedPath}`
}
