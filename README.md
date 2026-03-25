# LAN Chat

LAN chat app built with Go + Vue + WebSocket.

This repository currently has two run modes:

- Wails desktop mode (root `main.go` + `app.go`, HTTP API on `:5200`)
- Standalone backend mode (`backend/main.go`, HTTP API on `:8080`)

The frontend source is shared by both modes.

## Features

- Realtime chat over WebSocket
- File upload/download on local network
- Message and file history persisted in SQLite
- QR code endpoint for mobile access

## Requirements

- Go 1.21+
- Node.js 18+
- npm
- Wails CLI (for desktop build)

## Development

### Standalone backend + frontend

```bash
cd backend
go run main.go
```

```bash
cd frontend
npm install
npm run dev
```

Frontend dev server: `http://localhost:34115`  
Backend API: `http://localhost:8080`

### Desktop (Wails) mode

```bash
cd frontend
npm install
cd ..
wails dev
```

Desktop mode serves API from the embedded server on `http://127.0.0.1:5200`.

## Production Build (Desktop)

```bash
wails build -clean
```

## Notes

- Chat history and file metadata are stored in `data/lan_chat.db`.
- Uploaded files are stored in `uploads/`.
