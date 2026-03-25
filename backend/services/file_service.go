package services

import (
	"database/sql"
	"io"
	"lan-chat/common"
	"lan-chat/models"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type FileService struct {
	db        *sql.DB
	uploadDir string
}

func NewFileService() *FileService {
	uploadDir := "./uploads"
	os.MkdirAll(uploadDir, 0755)
	return &FileService{db: common.DB, uploadDir: uploadDir}
}

func (s *FileService) SaveFile(file *multipart.FileHeader, uploaderID, uploaderName string) (*models.File, error) {
	ext := filepath.Ext(file.Filename)
	storedName := uuid.New().String() + ext

	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	dst, err := os.Create(filepath.Join(s.uploadDir, storedName))
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	size, err := io.Copy(dst, src)
	if err != nil {
		return nil, err
	}

	record := &models.File{
		ID: uuid.New().String(), OriginalName: file.Filename, StoredName: storedName,
		FileSize: size, FileType: file.Header.Get("Content-Type"),
		UploaderID: uploaderID, UploaderName: uploaderName, CreatedAt: time.Now(),
	}

	_, err = s.db.Exec(
		"INSERT INTO files (id, original_name, stored_name, file_size, file_type, uploader_id, uploader_name, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		record.ID, record.OriginalName, record.StoredName, record.FileSize, record.FileType, record.UploaderID, record.UploaderName, record.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *FileService) GetFileByID(id string) (*models.File, error) {
	f := &models.File{}
	err := s.db.QueryRow(
		"SELECT id, original_name, stored_name, file_size, file_type, uploader_id, uploader_name, created_at FROM files WHERE id = ?", id,
	).Scan(&f.ID, &f.OriginalName, &f.StoredName, &f.FileSize, &f.FileType, &f.UploaderID, &f.UploaderName, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *FileService) GetFileList(page, pageSize int) ([]models.File, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	s.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&total)

	rows, err := s.db.Query(
		"SELECT id, original_name, stored_name, file_size, file_type, uploader_id, uploader_name, created_at FROM files ORDER BY created_at DESC LIMIT ? OFFSET ?",
		pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var f models.File
		if err := rows.Scan(&f.ID, &f.OriginalName, &f.StoredName, &f.FileSize, &f.FileType, &f.UploaderID, &f.UploaderName, &f.CreatedAt); err == nil {
			files = append(files, f)
		}
	}
	return files, total, nil
}

func (s *FileService) GetFilePath(storedName string) string {
	return filepath.Join(s.uploadDir, storedName)
}