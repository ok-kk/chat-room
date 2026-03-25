package models

import (
	"time"
)

type User struct {
	ID         string    `json:"id" db:"id"`
	Username   string    `json:"username" db:"username"`
	DeviceType string    `json:"device_type" db:"device_type"`
	DeviceName string    `json:"device_name" db:"device_name"`
	IPAddress  string    `json:"ip_address" db:"ip_address"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	LastSeen   time.Time `json:"last_seen" db:"last_seen"`
}

type Room struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Message struct {
	ID         string    `json:"id" db:"id"`
	RoomID     string    `json:"room_id" db:"room_id"`
	SenderID   string    `json:"sender_id" db:"sender_id"`
	SenderName string    `json:"sender_name" db:"sender_name"`
	Content    string    `json:"content" db:"content"`
	MsgType    string    `json:"msg_type" db:"msg_type"`
	FileURL    string    `json:"file_url,omitempty" db:"file_url"`
	FileName   string    `json:"file_name,omitempty" db:"file_name"`
	FileSize   int64     `json:"file_size,omitempty" db:"file_size"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type File struct {
	ID           string    `json:"id" db:"id"`
	OriginalName string    `json:"original_name" db:"original_name"`
	StoredName   string    `json:"stored_name" db:"stored_name"`
	FileSize     int64     `json:"file_size" db:"file_size"`
	FileType     string    `json:"file_type" db:"file_type"`
	UploaderID   string    `json:"uploader_id" db:"uploader_id"`
	UploaderName string    `json:"uploader_name" db:"uploader_name"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type LoginRequest struct {
	Username   string `json:"username" binding:"required"`
	DeviceType string `json:"device_type"`
	DeviceName string `json:"device_name"`
}

type LoginResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

type HistoryRequest struct {
	RoomID   string `json:"room_id" form:"room_id"`
	Page     int    `json:"page" form:"page"`
	PageSize int    `json:"page_size" form:"page_size"`
}

type HistoryResponse struct {
	Messages []Message `json:"messages"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}