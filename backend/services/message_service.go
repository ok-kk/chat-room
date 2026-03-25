package services

import (
	"database/sql"
	"lan-chat/common"
	"lan-chat/models"
	"time"

	"github.com/google/uuid"
)

type MessageService struct {
	db *sql.DB
}

func NewMessageService() *MessageService {
	return &MessageService{db: common.DB}
}

func (s *MessageService) SaveMessage(msg *models.Message) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	_, err := s.db.Exec(
		"INSERT INTO messages (id, room_id, sender_id, sender_name, content, msg_type, file_url, file_name, file_size, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		msg.ID, msg.RoomID, msg.SenderID, msg.SenderName, msg.Content, msg.MsgType, msg.FileURL, msg.FileName, msg.FileSize, msg.CreatedAt,
	)
	return err
}

func (s *MessageService) GetMessages(roomID string, page, pageSize int) ([]models.Message, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var total int
	s.db.QueryRow("SELECT COUNT(*) FROM messages WHERE room_id = ?", roomID).Scan(&total)

	rows, err := s.db.Query(
		"SELECT id, room_id, sender_id, sender_name, content, msg_type, file_url, file_name, file_size, created_at FROM messages WHERE room_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
		roomID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		var fileURL, fileName sql.NullString
		var fileSize sql.NullInt64
		err := rows.Scan(&msg.ID, &msg.RoomID, &msg.SenderID, &msg.SenderName, &msg.Content, &msg.MsgType, &fileURL, &fileName, &fileSize, &msg.CreatedAt)
		if err != nil {
			continue
		}
		if fileURL.Valid {
			msg.FileURL = fileURL.String
		}
		if fileName.Valid {
			msg.FileName = fileName.String
		}
		if fileSize.Valid {
			msg.FileSize = fileSize.Int64
		}
		messages = append(messages, msg)
	}
	return messages, total, nil
}