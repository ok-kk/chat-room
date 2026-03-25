package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"lan-chat/common"
	"lan-chat/models"
	"lan-chat/services"
	ws "lan-chat/websocket"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

//go:embed frontend/dist
var frontendFS embed.FS

var (
	messageService *services.MessageService
	fileService    *services.FileService
	upgrader       = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

func main() {
	// Setup log file
	logFile, err := os.OpenFile("lan-chat.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Panic: %v\n%s", r, debug.Stack())
			log.Println(errMsg)
			os.WriteFile("error.txt", []byte(errMsg), 0644)
			fmt.Println("Error! Check error.txt for details.")
			fmt.Println("Press Enter to exit...")
			fmt.Scanln()
		}
	}()

	fmt.Println("Starting LAN Chat Server...")

	gin.SetMode(gin.ReleaseMode)

	fmt.Println("Initializing database...")
	common.InitDB()
	common.InitTables()

	fmt.Println("Initializing services...")
	messageService = services.NewMessageService()
	fileService = services.NewFileService()

	hub := ws.NewHub()
	ws.GlobalHub = hub
	hub.OnMessage = saveMessage
	go hub.Run()

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	api := r.Group("/api")
	{
		api.GET("/health", handleHealth)
		api.POST("/login", handleLogin)
		api.GET("/users", handleGetUsers)
		api.GET("/messages", handleGetMessages)
		api.POST("/upload", handleFileUpload)
		api.GET("/files", handleGetFiles)
		api.GET("/download/:id", handleFileDownload)
		api.GET("/qrcode", handleQRCode)
		api.POST("/message/delete", handleDeleteMsgByContent)
		api.DELETE("/message/:id", handleDeleteMessage)
		api.DELETE("/file/:id", handleDeleteFile)
		api.GET("/thumbnail/:id", handleThumbnail)
	}

	r.GET("/ws", handleWebSocket)
	r.Static("/uploads", "./uploads")

	// Serve embedded frontend
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		errMsg := fmt.Sprintf("Frontend embed error: %v", err)
		log.Println(errMsg)
		fmt.Println(errMsg)
		fmt.Println("Make sure frontend/dist folder exists at build time.")
		fmt.Println("Press Enter to exit...")
		fmt.Scanln()
		os.Exit(1)
	}

	assetsFS, _ := fs.Sub(distFS, "assets")
	r.StaticFS("/assets", http.FS(assetsFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/" || path == "/index.html" {
			data, err := fs.ReadFile(distFS, "index.html")
			if err != nil {
				c.String(404, "Not Found")
				return
			}
			c.Data(200, "text/html; charset=utf-8", data)
			return
		}
		data, err := fs.ReadFile(distFS, path[1:])
		if err != nil {
			data, _ = fs.ReadFile(distFS, "index.html")
			c.Data(200, "text/html; charset=utf-8", data)
			return
		}
		c.Data(200, getContentType(path), data)
	})

	port := "8080"
	allIPs := common.GetAllLocalIPs()

	fmt.Println("")
	fmt.Println("========================================")
	fmt.Println("  LAN Chat Server Started!")
	fmt.Printf("  Local:  http://localhost:%s\n", port)
	fmt.Println("  LAN IPs:")
	for _, ip := range allIPs {
		fmt.Printf("    http://%s:%s\n", ip, port)
	}
	fmt.Println("========================================")
	fmt.Println("Close this window to stop the server.")
	fmt.Println("")

	log.Println("Server started on port", port)

	fmt.Println("")
	fmt.Println("Open browser and go to: http://localhost:" + port)
	fmt.Println("")

	if err := r.Run(":" + port); err != nil {
		errMsg := fmt.Sprintf("Server error: %v", err)
		log.Println(errMsg)
		fmt.Println(errMsg)
		fmt.Println("Press Enter to exit...")
		fmt.Scanln()
	}
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
		case ".gif":
			return "image/gif"
		case ".ttf":
			return "font/ttf"
		}
	}
	if n >= 5 {
		switch path[n-5:] {
		case ".json":
			return "application/json"
		case "2.css":
			return "text/css"
		}
	}
	if n >= 6 && path[n-6:] == ".woff2" {
		return "font/woff2"
	}
	return "application/octet-stream"
}

func saveMessage(msg *ws.Message) {
	if msg.Type != "chat" || msg.Content == "" {
		return
	}
	dbMsg := &models.Message{
		ID:         uuid.New().String(),
		RoomID:     msg.RoomID,
		SenderID:   msg.From,
		SenderName: msg.FromName,
		Content:    msg.Content,
		MsgType:    msg.MsgType,
		FileURL:    msg.FileURL,
		FileName:   msg.FileName,
		FileSize:   msg.FileSize,
		CreatedAt:  time.Now(),
	}
	if err := messageService.SaveMessage(dbMsg); err != nil {
		log.Println("Save message error:", err)
	}
	msg.Timestamp = dbMsg.CreatedAt.Format("2006-01-02 15:04:05")
}

func handleLogin(c *gin.Context) {
	var req struct {
		Username   string `json:"username" binding:"required"`
		DeviceType string `json:"device_type"`
		DeviceName string `json:"device_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	if req.DeviceType == "" {
		req.DeviceType = "web"
	}
	svc := services.NewUserService()
	user, err := svc.CreateOrUpdateUser(req.Username, req.DeviceType, req.DeviceName, c.ClientIP())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"token": user.ID, "user_id": user.ID, "user": user})
}

func handleGetUsers(c *gin.Context) {
	svc := services.NewUserService()
	users, err := svc.GetAllUsers()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, users)
}

func handleGetMessages(c *gin.Context) {
	roomID := c.DefaultQuery("room_id", "default")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	messages, total, err := messageService.GetMessages(roomID, page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"messages": messages, "total": total, "page": page, "page_size": pageSize})
}

func handleFileUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "No file"})
		return
	}
	uploaderID := c.PostForm("uploader_id")
	uploaderName := c.PostForm("uploader_name")
	record, err := fileService.SaveFile(file, uploaderID, uploaderName)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	fileURL := fmt.Sprintf("/api/download/%s", record.ID)
	msg := &models.Message{
		RoomID: "default", SenderID: uploaderID, SenderName: uploaderName,
		Content: fmt.Sprintf("Sent file: %s", record.OriginalName),
		MsgType: "file", FileURL: fileURL, FileName: record.OriginalName, FileSize: record.FileSize,
	}
	messageService.SaveMessage(msg)
	ws.GlobalHub.Broadcast <- &ws.Message{
		Type: "chat", From: uploaderID, FromName: uploaderName,
		Content: msg.Content, RoomID: "default", MsgType: "file",
		FileURL: fileURL, FileName: record.OriginalName, FileSize: record.FileSize,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}
	c.JSON(200, gin.H{"message": "OK", "file": record, "url": fileURL})
}

func handleGetFiles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	files, total, err := fileService.GetFileList(page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"files": files, "total": total, "page": page, "page_size": pageSize})
}

func handleFileDownload(c *gin.Context) {
	file, err := fileService.GetFileByID(c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "File not found"})
		return
	}
	c.FileAttachment(fileService.GetFilePath(file.StoredName), file.OriginalName)
}

func handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{"ok": true, "port": 8080})
}

func handleDeleteMessage(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(400, gin.H{"error": "Message ID required"})
		return
	}
	if _, err := common.DB.Exec("DELETE FROM messages WHERE id = ?", id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func handleDeleteMsgByContent(c *gin.Context) {
	var req struct {
		SenderID  string `json:"sender_id"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
		FileURL   string `json:"file_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.SenderID) == "" || strings.TrimSpace(req.CreatedAt) == "" {
		c.JSON(400, gin.H{"error": "Missing delete criteria"})
		return
	}

	if _, err := common.DB.Exec(
		"DELETE FROM messages WHERE sender_id = ? AND content = ? AND created_at = ?",
		req.SenderID, req.Content, req.CreatedAt,
	); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if fileID := extractFileID(req.FileURL); fileID != "" {
		if storedName, err := lookupStoredFileName(fileID); err == nil && storedName != "" {
			_ = os.Remove(filepath.Join("./uploads", storedName))
			_ = os.Remove(filepath.Join("./uploads", "thumb_"+storedName))
		}
		_, _ = common.DB.Exec("DELETE FROM files WHERE id = ?", fileID)
	}

	c.JSON(200, gin.H{"success": true})
}

func handleDeleteFile(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(400, gin.H{"error": "File ID required"})
		return
	}

	storedName, _ := lookupStoredFileName(id)
	if storedName != "" {
		_ = os.Remove(filepath.Join("./uploads", storedName))
		_ = os.Remove(filepath.Join("./uploads", "thumb_"+storedName))
	}

	if _, err := common.DB.Exec("DELETE FROM files WHERE id = ?", id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if _, err := common.DB.Exec("DELETE FROM messages WHERE file_url LIKE ?", "%"+id+"%"); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func handleThumbnail(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.String(400, "Bad Request")
		return
	}

	file, err := fileService.GetFileByID(id)
	if err != nil {
		c.String(404, "Not Found")
		return
	}

	srcPath := fileService.GetFilePath(file.StoredName)
	thumbPath := fileService.GetFilePath("thumb_" + file.StoredName)

	if _, err := os.Stat(thumbPath); err == nil {
		c.File(thumbPath)
		return
	}

	if isImage(file.FileType, file.StoredName) && generateThumbnail(srcPath, thumbPath) {
		c.File(thumbPath)
		return
	}

	c.File(srcPath)
}

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &ws.Client{
		ID:     c.Query("user_id"),
		Name:   c.Query("user_name"),
		Conn:   conn,
		Send:   make(chan []byte, 256),
		RoomID: c.DefaultQuery("room_id", "default"),
	}
	ws.GlobalHub.Register <- client
	go client.WritePump()
	go client.ReadPump(ws.GlobalHub)
}

func handleQRCode(c *gin.Context) {
	allIPs := common.GetAllLocalIPs()
	urls := make([]string, len(allIPs))
	for i, ip := range allIPs {
		urls[i] = fmt.Sprintf("http://%s:8080", ip)
	}
	best := "http://localhost:8080"
	if len(urls) > 0 {
		best = urls[0]
	}
	c.JSON(200, gin.H{"url": best, "all_urls": urls, "all_ips": allIPs})
}

func extractFileID(fileURL string) string {
	fileURL = strings.TrimSpace(fileURL)
	if fileURL == "" {
		return ""
	}
	lastSlash := strings.LastIndex(fileURL, "/")
	if lastSlash == -1 || lastSlash == len(fileURL)-1 {
		return ""
	}
	return fileURL[lastSlash+1:]
}

func lookupStoredFileName(fileID string) (string, error) {
	var storedName string
	err := common.DB.QueryRow("SELECT stored_name FROM files WHERE id = ?", fileID).Scan(&storedName)
	return storedName, err
}

func isImage(contentType, filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return strings.HasPrefix(contentType, "image/") ||
		ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".bmp" || ext == ".webp"
}

func generateThumbnail(src, dst string) bool {
	srcFile, err := os.Open(src)
	if err != nil {
		return false
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return false
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return false
	}
	return true
}
