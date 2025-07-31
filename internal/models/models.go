package models

import (
	"time"
)

type AdminUser struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type APIKey struct {
	ID        int        `json:"id"`
	KeyHash   string     `json:"-"`
	Name      string     `json:"name"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}

type APIRequest struct {
	ID         int       `json:"id"`
	APIKeyID   int       `json:"api_key_id"`
	Timestamp  time.Time `json:"timestamp"`
	ImageCount int       `json:"image_count"`
}

type ImageFile struct {
	ID       int    `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
	Enabled  bool   `json:"enabled"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Setting struct {
	ID    int    `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}