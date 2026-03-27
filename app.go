package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	_ "modernc.org/sqlite"
)

//go:embed frontend/dist
var frontendFS embed.FS

type User struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	DeviceType string `json:"device_type"`
	IPAddress  string `json:"ip_address"`
	CreatedAt  string `json:"created_at"`
}

type Message struct {
	ID         string `json:"id"`
	RoomID     string `json:"room_id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Content    string `json:"content"`
	MsgType    string `json:"msg_type"`
	FileURL    string `json:"file_url,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	FileSize   int64  `json:"file_size,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type FileRecord struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
	StoredName   string `json:"stored_name"`
	FileSize     int64  `json:"file_size"`
	FileType     string `json:"file_type"`
	UploaderID   string `json:"uploader_id"`
	UploaderName string `json:"uploader_name"`
	CreatedAt    string `json:"created_at"`
}

type WSMessage struct {
	ID        string      `json:"id,omitempty"`
	Type      string      `json:"type"`
	From      string      `json:"from"`
	FromName  string      `json:"from_name"`
	Content   string      `json:"content"`
	RoomID    string      `json:"room_id"`
	MsgType   string      `json:"msg_type"`
	FileURL   string      `json:"file_url,omitempty"`
	FileName  string      `json:"file_name,omitempty"`
	FileSize  int64       `json:"file_size,omitempty"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

type deleteMessageRequest struct {
	ID        string
	SenderID  string
	Content   string
	CreatedAt string
	FileURL   string
}

type WSClient struct {
	ID     string
	Name   string
	Conn   *websocket.Conn
	Send   chan []byte
	RoomID string
}

type Hub struct {
	Clients    map[string]*WSClient
	Rooms      map[string]map[string]*WSClient
	Broadcast  chan *WSMessage
	Register   chan *WSClient
	Unregister chan *WSClient
	mu         sync.RWMutex
}

type App struct {
	ctx       context.Context
	db        *sql.DB
	hub       *Hub
	server    *http.Server
	storageDir string
	uploadDir  string
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initPaths()
	a.initDB()
	a.initHub()
	a.startHTTPServer()
}

func (a *App) initPaths() {
	baseDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(baseDir) == "" {
		baseDir, err = os.Getwd()
		if err != nil {
			log.Fatal("storage path error:", err)
		}
	}

	a.storageDir = filepath.Join(baseDir, "lan-chat")
	a.uploadDir = filepath.Join(a.storageDir, "uploads")

	if err := os.MkdirAll(a.uploadDir, 0755); err != nil {
		log.Fatal("storage init error:", err)
	}
}

func (a *App) dbPath() string {
	return filepath.Join(a.storageDir, "lan_chat.db")
}

func (a *App) initDB() {
	var err error
	a.db, err = sql.Open("sqlite", a.dbPath()+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal("DB open error:", err)
	}
	a.db.SetMaxOpenConns(1)
	a.db.SetMaxIdleConns(1)

	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE,
			device_type TEXT DEFAULT 'web', ip_address TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY, room_id TEXT DEFAULT 'default',
			sender_id TEXT NOT NULL, sender_name TEXT NOT NULL,
			content TEXT DEFAULT '', msg_type TEXT DEFAULT 'text',
			file_url TEXT DEFAULT '', file_name TEXT DEFAULT '',
			file_size INTEGER DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY, original_name TEXT NOT NULL,
			stored_name TEXT NOT NULL, file_size INTEGER DEFAULT 0,
			file_type TEXT DEFAULT '', uploader_id TEXT NOT NULL,
			uploader_name TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, t := range tables {
		a.db.Exec(t)
	}
	log.Println("Database ready:", a.dbPath())
}

func (a *App) initHub() {
	a.hub = &Hub{
		Clients:    make(map[string]*WSClient),
		Rooms:      make(map[string]map[string]*WSClient),
		Broadcast:  make(chan *WSMessage, 256),
		Register:   make(chan *WSClient),
		Unregister: make(chan *WSClient),
	}
	go a.hub.run(a)
}

func (h *Hub) run(app *App) {
	for {
		select {
		case c := <-h.Register:
			h.mu.Lock()
			h.Clients[c.ID] = c
			if h.Rooms[c.RoomID] == nil {
				h.Rooms[c.RoomID] = make(map[string]*WSClient)
			}
			h.Rooms[c.RoomID][c.ID] = c
			h.mu.Unlock()
			h.broadcastUserList(c.RoomID)
		case c := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[c.ID]; ok {
				delete(h.Clients, c.ID)
				if room, ok := h.Rooms[c.RoomID]; ok {
					delete(room, c.ID)
				}
				close(c.Send)
			}
			h.mu.Unlock()
			h.broadcastUserList(c.RoomID)
		case msg := <-h.Broadcast:
			if msg.Type == "chat" && msg.Content != "" {
				app.saveMessage(msg)
			}
			h.mu.Lock()
			if room, ok := h.Rooms[msg.RoomID]; ok {
				data, _ := json.Marshal(msg)
				for _, c := range room {
					select {
					case c.Send <- data:
					default:
						close(c.Send)
						delete(h.Clients, c.ID)
						delete(room, c.ID)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) broadcastUserList(roomID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users := []map[string]string{}
	if room, ok := h.Rooms[roomID]; ok {
		for _, c := range room {
			users = append(users, map[string]string{"id": c.ID, "name": c.Name})
		}
	}
	msg := &WSMessage{Type: "user_list", RoomID: roomID, Data: users}
	if room, ok := h.Rooms[roomID]; ok {
		data, _ := json.Marshal(msg)
		for _, c := range room {
			select {
			case c.Send <- data:
			default:
			}
		}
	}
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (a *App) startHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", a.handleLogin)
	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/messages", a.handleGetMessages)
	mux.HandleFunc("/api/upload", a.handleUpload)
	mux.HandleFunc("/api/download/", a.handleDownload)
	mux.HandleFunc("/api/files", a.handleGetFiles)
	mux.HandleFunc("/api/qrcode", a.handleQRCode)
	mux.HandleFunc("/api/message/delete", a.handleDeleteMsgByContent)
	mux.HandleFunc("/api/message/", a.handleDeleteMessage)
	mux.HandleFunc("/api/file/", a.handleDeleteFile)
	mux.HandleFunc("/api/thumbnail/", a.handleThumbnail)
	mux.HandleFunc("/ws", a.handleWS)
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(a.uploadDir))))

	distFS, _ := fs.Sub(frontendFS, "frontend/dist")
	assetsFS, _ := fs.Sub(distFS, "assets")
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/api") && !strings.HasPrefix(r.URL.Path, "/ws") && !strings.HasPrefix(r.URL.Path, "/uploads") {
			data, err := fs.ReadFile(distFS, r.URL.Path[1:])
			if err == nil {
				w.Header().Set("Content-Type", getContentType(r.URL.Path))
				w.Write(data)
				return
			}
		}
		data, _ := fs.ReadFile(distFS, "index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	a.server = &http.Server{Addr: ":5200", Handler: mux}
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("HTTP server error:", err)
		}
	}()
}

func getContentType(path string) string {
	n := len(path)
	if n >= 3 && path[n-3:] == ".js" {
		return "application/javascript"
	}
	if n >= 4 {
		switch path[n-4:] {
		case ".css":
			return "text/css"
		case ".svg":
			return "image/svg+xml"
		case ".png":
			return "image/png"
		case ".ico":
			return "image/x-icon"
		}
	}
	if n >= 5 && path[n-5:] == ".json" {
		return "application/json"
	}
	if n >= 6 && path[n-6:] == ".woff2" {
		return "font/woff2"
	}
	return "application/octet-stream"
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "port": 5200})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	var req struct {
		Username   string `json:"username"`
		DeviceType string `json:"device_type"`
		DeviceName string `json:"device_name"`
		IPAddress  string `json:"ip_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Username required"})
		return
	}
	if req.DeviceType == "" {
		req.DeviceType = "web"
	}
	if req.IPAddress == "" {
		req.IPAddress = clientIP(r)
	}
	var id string
	err := a.db.QueryRow("SELECT id FROM users WHERE username=?", req.Username).Scan(&id)
	if id == "" {
		id = uuid.New().String()
		_, err = a.db.Exec("INSERT INTO users(id,username,device_type,ip_address,created_at) VALUES(?,?,?,?,?)", id, req.Username, req.DeviceType, req.IPAddress, time.Now().Format("2006-01-02 15:04:05"))
	} else if err == nil {
		_, err = a.db.Exec("UPDATE users SET device_type=?, ip_address=? WHERE id=?", req.DeviceType, req.IPAddress, id)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"token": id, "user_id": id, "user": map[string]string{"id": id, "username": req.Username, "device_type": req.DeviceType, "ip_address": req.IPAddress}})
}

func (a *App) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		roomID = "default"
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize
	var total int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM messages WHERE room_id=?", roomID).Scan(&total); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	rows, err := a.db.Query("SELECT id,room_id,sender_id,sender_name,content,msg_type,file_url,file_name,file_size,created_at FROM messages WHERE room_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?", roomID, pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	msgs := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.SenderName, &m.Content, &m.MsgType, &m.FileURL, &m.FileName, &m.FileSize, &m.CreatedAt); err == nil {
			msgs = append(msgs, m)
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"messages": msgs, "total": total, "page": page, "page_size": pageSize})
}

func (a *App) handleGetFiles(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	var total int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&total); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	rows, err := a.db.Query("SELECT id,original_name,stored_name,file_size,file_type,uploader_id,uploader_name,created_at FROM files ORDER BY created_at DESC LIMIT ? OFFSET ?", pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	files := []FileRecord{}
	for rows.Next() {
		var f FileRecord
		if err := rows.Scan(&f.ID, &f.OriginalName, &f.StoredName, &f.FileSize, &f.FileType, &f.UploaderID, &f.UploaderName, &f.CreatedAt); err == nil {
			files = append(files, f)
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"files": files, "total": total, "page": page, "page_size": pageSize})
}

func (a *App) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &WSClient{ID: r.URL.Query().Get("user_id"), Name: r.URL.Query().Get("user_name"), Conn: conn, Send: make(chan []byte, 256), RoomID: "default"}
	a.hub.Register <- client
	go func() {
		defer func() {
			a.hub.Unregister <- client
			conn.Close()
		}()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var wsMsg WSMessage
			if json.Unmarshal(msg, &wsMsg) == nil {
				wsMsg.From = client.ID
				wsMsg.FromName = client.Name
				if wsMsg.RoomID == "" {
					wsMsg.RoomID = "default"
				}
				a.hub.Broadcast <- &wsMsg
			}
		}
	}()
	go func() {
		defer conn.Close()
		for msg := range client.Send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()
}

func (a *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(100 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file", 400)
		return
	}
	defer file.Close()
	uploaderID := r.FormValue("uploader_id")
	uploaderName := r.FormValue("uploader_name")
	storedName := uuid.New().String() + filepath.Ext(header.Filename)
	dst, err := os.Create(filepath.Join(a.uploadDir, storedName))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer dst.Close()
	size, err := io.Copy(dst, file)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	record := FileRecord{ID: uuid.New().String(), OriginalName: header.Filename, StoredName: storedName, FileSize: size, FileType: header.Header.Get("Content-Type"), UploaderID: uploaderID, UploaderName: uploaderName, CreatedAt: time.Now().Format("2006-01-02 15:04:05")}
	a.db.Exec("INSERT INTO files VALUES(?,?,?,?,?,?,?,?)", record.ID, record.OriginalName, record.StoredName, record.FileSize, record.FileType, record.UploaderID, record.UploaderName, record.CreatedAt)
	fileURL := "/api/download/" + record.ID
	a.hub.Broadcast <- &WSMessage{Type: "chat", From: uploaderID, FromName: uploaderName, Content: "Sent file: " + header.Filename, RoomID: "default", MsgType: "file", FileURL: fileURL, FileName: header.Filename, FileSize: size, Timestamp: record.CreatedAt}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "url": fileURL, "name": header.Filename})
}

func (a *App) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/download/"):]
	var storedName, originalName string
	if err := a.db.QueryRow("SELECT stored_name, original_name FROM files WHERE id=?", id).Scan(&storedName, &originalName); err != nil {
		http.Error(w, "Not found", 404)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", originalName))
	http.ServeFile(w, r, filepath.Join(a.uploadDir, storedName))
}

func (a *App) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	id := r.URL.Path[len("/api/message/"):]
	a.db.Exec("DELETE FROM messages WHERE id=?", id)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (a *App) handleDeleteMsgByContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	var req struct {
		ID        string `json:"id"`
		SenderID  string `json:"sender_id"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
		FileURL   string `json:"file_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}
	if err := a.deleteMessage(deleteMessageRequest{ID: req.ID, SenderID: req.SenderID, Content: req.Content, CreatedAt: req.CreatedAt, FileURL: req.FileURL}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (a *App) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	id := r.URL.Path[len("/api/file/"):]
	var storedName string
	a.db.QueryRow("SELECT stored_name FROM files WHERE id=?", id).Scan(&storedName)
	if storedName != "" {
		_ = os.Remove(filepath.Join(a.uploadDir, storedName))
		_ = os.Remove(filepath.Join(a.uploadDir, "thumb_"+storedName))
	}
	a.db.Exec("DELETE FROM files WHERE id=?", id)
	a.db.Exec("DELETE FROM messages WHERE file_url LIKE ?", "%"+id+"%")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (a *App) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/thumbnail/"):]
	var storedName, fileType string
	if err := a.db.QueryRow("SELECT stored_name, file_type FROM files WHERE id=?", id).Scan(&storedName, &fileType); err != nil {
		http.Error(w, "Not found", 404)
		return
	}
	thumbPath := filepath.Join(a.uploadDir, "thumb_"+storedName)
	if _, err := os.Stat(thumbPath); err == nil {
		http.ServeFile(w, r, thumbPath)
		return
	}
	srcPath := filepath.Join(a.uploadDir, storedName)
	if isImage(fileType, storedName) && generateThumbnail(srcPath, thumbPath) {
		http.ServeFile(w, r, thumbPath)
		return
	}
	http.ServeFile(w, r, srcPath)
}

func isImage(contentType, filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return strings.HasPrefix(contentType, "image/") || ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".bmp" || ext == ".webp"
}

func generateThumbnail(src, dst string) bool {
	in, err := os.Open(src)
	if err != nil {
		return false
	}
	defer in.Close()

	img, format, err := image.Decode(in)
	if err != nil {
		return false
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width == 0 || height == 0 {
		return false
	}

	const maxSize = 320
	scale := 1.0
	if width > maxSize || height > maxSize {
		if width >= height {
			scale = float64(maxSize) / float64(width)
		} else {
			scale = float64(maxSize) / float64(height)
		}
	}
	newW := int(float64(width) * scale)
	newH := int(float64(height) * scale)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	thumb := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := bounds.Min.Y + y*height/newH
		for x := 0; x < newW; x++ {
			srcX := bounds.Min.X + x*width/newW
			thumb.Set(x, y, img.At(srcX, srcY))
		}
	}

	out, err := os.Create(dst)
	if err != nil {
		return false
	}
	defer out.Close()

	switch format {
	case "jpeg", "jpg":
		return jpeg.Encode(out, thumb, &jpeg.Options{Quality: 80}) == nil
	default:
		return png.Encode(out, thumb) == nil
	}
}

func (a *App) handleQRCode(w http.ResponseWriter, r *http.Request) {
	allIPs := GetAllLocalIPs()
	urls := make([]string, 0, len(allIPs))
	for _, ip := range allIPs {
		urls = append(urls, fmt.Sprintf("http://%s:5200", ip))
	}
	best := "http://localhost:5200"
	if len(urls) > 0 {
		best = urls[0]
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"url": best,
		"all_urls": urls,
		"all_ips": allIPs,
	})
}
func (a *App) Login(username string) map[string]interface{} {
	if username == "" {
		return map[string]interface{}{"error": "Username required"}
	}
	var id string
	a.db.QueryRow("SELECT id FROM users WHERE username=?", username).Scan(&id)
	if id == "" {
		id = uuid.New().String()
		ip := GetLocalIP()
		a.db.Exec("INSERT INTO users(id,username,device_type,ip_address,created_at) VALUES(?,?,?,?,?)", id, username, "desktop", ip, time.Now().Format("2006-01-02 15:04:05"))
	}
	return map[string]interface{}{"id": id, "username": username}
}

func (a *App) GetMessages(roomID string, page int) []Message {
	if roomID == "" {
		roomID = "default"
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 50
	rows, err := a.db.Query("SELECT id,room_id,sender_id,sender_name,content,msg_type,file_url,file_name,file_size,created_at FROM messages WHERE room_id=? ORDER BY created_at DESC LIMIT 50 OFFSET ?", roomID, offset)
	if err != nil {
		return []Message{}
	}
	defer rows.Close()
	msgs := []Message{}
	for rows.Next() {
		var m Message
		if rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.SenderName, &m.Content, &m.MsgType, &m.FileURL, &m.FileName, &m.FileSize, &m.CreatedAt) == nil {
			msgs = append(msgs, m)
		}
	}
	return msgs
}

func (a *App) GetFiles(page int) []FileRecord {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 20
	rows, err := a.db.Query("SELECT id,original_name,stored_name,file_size,file_type,uploader_id,uploader_name,created_at FROM files ORDER BY created_at DESC LIMIT 20 OFFSET ?", offset)
	if err != nil {
		return []FileRecord{}
	}
	defer rows.Close()
	files := []FileRecord{}
	for rows.Next() {
		var f FileRecord
		if rows.Scan(&f.ID, &f.OriginalName, &f.StoredName, &f.FileSize, &f.FileType, &f.UploaderID, &f.UploaderName, &f.CreatedAt) == nil {
			files = append(files, f)
		}
	}
	return files
}

func (a *App) GetServerURL() string {
	return fmt.Sprintf("http://%s:5200", GetLocalIP())
}

func (a *App) GetAllIPs() []string {
	return GetAllLocalIPs()
}

func (a *App) SaveUploadedFile(filePath string, uploaderID string, uploaderName string) map[string]interface{} {
	srcFile, err := os.Open(filePath)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	defer srcFile.Close()
	info, _ := srcFile.Stat()
	storedName := uuid.New().String() + filepath.Ext(info.Name())
	dst, err := os.Create(filepath.Join(a.uploadDir, storedName))
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	defer dst.Close()
	size, err := io.Copy(dst, srcFile)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	record := FileRecord{ID: uuid.New().String(), OriginalName: info.Name(), StoredName: storedName, FileSize: size, FileType: mimeTypeFromName(info.Name()), UploaderID: uploaderID, UploaderName: uploaderName, CreatedAt: time.Now().Format("2006-01-02 15:04:05")}
	a.db.Exec("INSERT INTO files VALUES(?,?,?,?,?,?,?,?)", record.ID, record.OriginalName, record.StoredName, record.FileSize, record.FileType, record.UploaderID, record.UploaderName, record.CreatedAt)
	fileURL := "/api/download/" + record.ID
	a.hub.Broadcast <- &WSMessage{Type: "chat", From: uploaderID, FromName: uploaderName, Content: "Sent file: " + info.Name(), RoomID: "default", MsgType: "file", FileURL: fileURL, FileName: info.Name(), FileSize: size, Timestamp: record.CreatedAt}
	return map[string]interface{}{"success": true, "url": fileURL}
}

func mimeTypeFromName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func (a *App) DeleteMessage(id string, senderID string, content string, createdAt string, fileURL string) map[string]interface{} {
	if err := a.deleteMessage(deleteMessageRequest{ID: id, SenderID: senderID, Content: content, CreatedAt: createdAt, FileURL: fileURL}); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"success": true}
}

func (a *App) saveMessage(msg *WSMessage) {
	id := uuid.New().String()
	if msg.Timestamp == "" {
		msg.Timestamp = time.Now().Format("2006-01-02 15:04:05")
	}
	msg.ID = id
	a.db.Exec("INSERT INTO messages(id,room_id,sender_id,sender_name,content,msg_type,file_url,file_name,file_size,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)", id, msg.RoomID, msg.From, msg.FromName, msg.Content, msg.MsgType, msg.FileURL, msg.FileName, msg.FileSize, msg.Timestamp)
}

func (a *App) deleteMessage(req deleteMessageRequest) error {
	req.ID = strings.TrimSpace(req.ID)
	req.SenderID = strings.TrimSpace(req.SenderID)
	req.Content = strings.TrimSpace(req.Content)
	req.CreatedAt = strings.TrimSpace(req.CreatedAt)
	req.FileURL = strings.TrimSpace(req.FileURL)
	if req.ID == "" && (req.SenderID == "" || req.CreatedAt == "") {
		return fmt.Errorf("missing delete criteria")
	}
	var messageID, roomID, fileURL string
	if req.ID != "" {
		err := a.db.QueryRow("SELECT id, room_id, COALESCE(file_url, '') FROM messages WHERE id=?", req.ID).Scan(&messageID, &roomID, &fileURL)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
	}
	if messageID == "" {
		err := a.db.QueryRow("SELECT id, room_id, COALESCE(file_url, '') FROM messages WHERE sender_id=? AND content=? AND created_at=? LIMIT 1", req.SenderID, req.Content, req.CreatedAt).Scan(&messageID, &roomID, &fileURL)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("message not found")
			}
			return err
		}
	}
	if req.FileURL != "" {
		fileURL = req.FileURL
	}
	if roomID == "" {
		roomID = "default"
	}
	if _, err := a.db.Exec("DELETE FROM messages WHERE id=?", messageID); err != nil {
		return err
	}
	if fileURL != "" {
		fileID := fileURL[strings.LastIndex(fileURL, "/")+1:]
		var storedName string
		_ = a.db.QueryRow("SELECT stored_name FROM files WHERE id=?", fileID).Scan(&storedName)
		if storedName != "" {
			_ = os.Remove(filepath.Join(a.uploadDir, storedName))
			_ = os.Remove(filepath.Join(a.uploadDir, "thumb_"+storedName))
		}
		_, _ = a.db.Exec("DELETE FROM files WHERE id=?", fileID)
	}
	a.hub.Broadcast <- &WSMessage{Type: "message_deleted", ID: messageID, RoomID: roomID, FileURL: fileURL, Timestamp: time.Now().Format("2006-01-02 15:04:05"), Data: map[string]string{"id": messageID, "sender_id": req.SenderID, "content": req.Content, "created_at": req.CreatedAt, "file_url": fileURL}}
	return nil
}

func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

