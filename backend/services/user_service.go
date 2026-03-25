package services

import (
	"database/sql"
	"lan-chat/common"
	"lan-chat/models"
	"time"

	"github.com/google/uuid"
)

type UserService struct {
	db *sql.DB
}

func NewUserService() *UserService {
	return &UserService{db: common.DB}
}

func (s *UserService) CreateOrUpdateUser(username, deviceType, deviceName, ipAddress string) (*models.User, error) {
	now := time.Now()

	var existingID string
	err := s.db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&existingID)

	if err == nil {
		_, err = s.db.Exec(
			"UPDATE users SET device_type=?, device_name=?, ip_address=?, last_seen=? WHERE id=?",
			deviceType, deviceName, ipAddress, now, existingID,
		)
		if err != nil {
			return nil, err
		}
		return s.GetUserByID(existingID)
	}

	id := uuid.New().String()
	_, err = s.db.Exec(
		"INSERT INTO users (id, username, device_type, device_name, ip_address, created_at, last_seen) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, username, deviceType, deviceName, ipAddress, now, now,
	)
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID: id, Username: username, DeviceType: deviceType,
		DeviceName: deviceName, IPAddress: ipAddress,
		CreatedAt: now, LastSeen: now,
	}, nil
}

func (s *UserService) GetUserByID(id string) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(
		"SELECT id, username, device_type, device_name, ip_address, created_at, last_seen FROM users WHERE id = ?", id,
	).Scan(&user.ID, &user.Username, &user.DeviceType, &user.DeviceName, &user.IPAddress, &user.CreatedAt, &user.LastSeen)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetAllUsers() ([]models.User, error) {
	rows, err := s.db.Query("SELECT id, username, device_type, device_name, ip_address, created_at, last_seen FROM users ORDER BY last_seen DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.DeviceType, &u.DeviceName, &u.IPAddress, &u.CreatedAt, &u.LastSeen); err == nil {
			users = append(users, u)
		}
	}
	return users, nil
}