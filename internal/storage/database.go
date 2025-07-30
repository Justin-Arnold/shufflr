package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"shufflr/internal/models"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	conn *sql.DB
}

func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS admin_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_hash TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_used DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS api_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			api_key_id INTEGER NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			image_count INTEGER NOT NULL,
			FOREIGN KEY (api_key_id) REFERENCES api_keys (id)
		)`,
		`CREATE TABLE IF NOT EXISTS image_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			filename TEXT UNIQUE NOT NULL,
			size INTEGER NOT NULL,
			mime_type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 1,
			uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_requests_key_id ON api_requests(api_key_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_requests_timestamp ON api_requests(timestamp)`,
		// Add enabled column to existing image_files table if it doesn't exist
		`ALTER TABLE image_files ADD COLUMN enabled BOOLEAN DEFAULT 1`,
	}

	for _, query := range queries {
		if _, err := db.conn.Exec(query); err != nil {
			return fmt.Errorf("failed to execute migration query: %w", err)
		}
	}

	return nil
}

// Admin User methods
func (db *DB) CreateAdminUser(username, password string) (*models.AdminUser, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	query := `INSERT INTO admin_users (username, password_hash) VALUES (?, ?)`
	result, err := db.conn.Exec(query, username, string(hashedPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &models.AdminUser{
		ID:           int(id),
		Username:     username,
		PasswordHash: string(hashedPassword),
		CreatedAt:    time.Now(),
	}, nil
}

func (db *DB) GetAdminUserByUsername(username string) (*models.AdminUser, error) {
	query := `SELECT id, username, password_hash, created_at FROM admin_users WHERE username = ?`
	row := db.conn.QueryRow(query, username)

	var user models.AdminUser
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get admin user: %w", err)
	}

	return &user, nil
}

func (db *DB) HasAdminUsers() (bool, error) {
	query := `SELECT COUNT(*) FROM admin_users`
	var count int
	err := db.conn.QueryRow(query).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to count admin users: %w", err)
	}
	return count > 0, nil
}

// API Key methods
func (db *DB) CreateAPIKey(name string) (*models.APIKey, string, error) {
	// Generate random API key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}
	apiKey := hex.EncodeToString(keyBytes)

	// Hash the key for storage
	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])

	query := `INSERT INTO api_keys (key_hash, name) VALUES (?, ?)`
	result, err := db.conn.Exec(query, keyHash, name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create API key: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &models.APIKey{
		ID:        int(id),
		KeyHash:   keyHash,
		Name:      name,
		Enabled:   true,
		CreatedAt: time.Now(),
	}, apiKey, nil
}

func (db *DB) GetAPIKeyByKey(apiKey string) (*models.APIKey, error) {
	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])

	query := `SELECT id, key_hash, name, enabled, created_at, last_used FROM api_keys WHERE key_hash = ? AND enabled = 1`
	row := db.conn.QueryRow(query, keyHash)

	var key models.APIKey
	var lastUsed sql.NullTime
	err := row.Scan(&key.ID, &key.KeyHash, &key.Name, &key.Enabled, &key.CreatedAt, &lastUsed)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	if lastUsed.Valid {
		key.LastUsed = &lastUsed.Time
	}

	return &key, nil
}

func (db *DB) GetAllAPIKeys() ([]*models.APIKey, error) {
	query := `SELECT id, key_hash, name, enabled, created_at, last_used FROM api_keys ORDER BY created_at DESC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get API keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.APIKey
	for rows.Next() {
		var key models.APIKey
		var lastUsed sql.NullTime
		err := rows.Scan(&key.ID, &key.KeyHash, &key.Name, &key.Enabled, &key.CreatedAt, &lastUsed)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}

		if lastUsed.Valid {
			key.LastUsed = &lastUsed.Time
		}

		keys = append(keys, &key)
	}

	return keys, nil
}

func (db *DB) UpdateAPIKeyLastUsed(keyID int) error {
	query := `UPDATE api_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.conn.Exec(query, keyID)
	if err != nil {
		return fmt.Errorf("failed to update API key last used: %w", err)
	}
	return nil
}

func (db *DB) UpdateAPIKeyEnabled(keyID int, enabled bool) error {
	query := `UPDATE api_keys SET enabled = ? WHERE id = ?`
	_, err := db.conn.Exec(query, enabled, keyID)
	if err != nil {
		return fmt.Errorf("failed to update API key enabled status: %w", err)
	}
	return nil
}

func (db *DB) DeleteAPIKey(keyID int) error {
	query := `DELETE FROM api_keys WHERE id = ?`
	_, err := db.conn.Exec(query, keyID)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	return nil
}

// API Request methods
func (db *DB) LogAPIRequest(keyID, imageCount int) error {
	query := `INSERT INTO api_requests (api_key_id, image_count) VALUES (?, ?)`
	_, err := db.conn.Exec(query, keyID, imageCount)
	if err != nil {
		return fmt.Errorf("failed to log API request: %w", err)
	}
	return nil
}

func (db *DB) GetAPIKeyUsageCount(keyID int) (int, error) {
	query := `SELECT COUNT(*) FROM api_requests WHERE api_key_id = ?`
	var count int
	err := db.conn.QueryRow(query, keyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get API key usage count: %w", err)
	}
	return count, nil
}

// Image File methods
func (db *DB) CreateImageFile(filename string, size int64, mimeType string) (*models.ImageFile, error) {
	query := `INSERT INTO image_files (filename, size, mime_type, enabled) VALUES (?, ?, ?, ?)`
	result, err := db.conn.Exec(query, filename, size, mimeType, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create image file record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &models.ImageFile{
		ID:         int(id),
		Filename:   filename,
		Size:       size,
		MimeType:   mimeType,
		Enabled:    true,
		UploadedAt: time.Now(),
	}, nil
}

func (db *DB) GetAllImageFiles() ([]*models.ImageFile, error) {
	query := `SELECT id, filename, size, mime_type, enabled, uploaded_at FROM image_files ORDER BY uploaded_at DESC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get image files: %w", err)
	}
	defer rows.Close()

	var images []*models.ImageFile
	for rows.Next() {
		var img models.ImageFile
		err := rows.Scan(&img.ID, &img.Filename, &img.Size, &img.MimeType, &img.Enabled, &img.UploadedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan image file: %w", err)
		}
		images = append(images, &img)
	}

	return images, nil
}

func (db *DB) GetRandomImageFiles(count int) ([]*models.ImageFile, error) {
	query := `SELECT id, filename, size, mime_type, enabled, uploaded_at FROM image_files WHERE enabled = 1 ORDER BY RANDOM() LIMIT ?`
	rows, err := db.conn.Query(query, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get random image files: %w", err)
	}
	defer rows.Close()

	var images []*models.ImageFile
	for rows.Next() {
		var img models.ImageFile
		err := rows.Scan(&img.ID, &img.Filename, &img.Size, &img.MimeType, &img.Enabled, &img.UploadedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan image file: %w", err)
		}
		images = append(images, &img)
	}

	return images, nil
}

func (db *DB) GetImageFileCount() (int, error) {
	query := `SELECT COUNT(*) FROM image_files WHERE enabled = 1`
	var count int
	err := db.conn.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get image file count: %w", err)
	}
	return count, nil
}

func (db *DB) DeleteImageFile(filename string) error {
	query := `DELETE FROM image_files WHERE filename = ?`
	_, err := db.conn.Exec(query, filename)
	if err != nil {
		return fmt.Errorf("failed to delete image file record: %w", err)
	}
	return nil
}

func (db *DB) UpdateImageFilename(oldFilename, newFilename string) error {
	query := `UPDATE image_files SET filename = ? WHERE filename = ?`
	_, err := db.conn.Exec(query, newFilename, oldFilename)
	if err != nil {
		return fmt.Errorf("failed to update image filename: %w", err)
	}
	return nil
}

func (db *DB) UpdateImageEnabled(filename string, enabled bool) error {
	query := `UPDATE image_files SET enabled = ? WHERE filename = ?`
	_, err := db.conn.Exec(query, enabled, filename)
	if err != nil {
		return fmt.Errorf("failed to update image enabled status: %w", err)
	}
	return nil
}

func (db *DB) GetTotalImageFileCount() (int, error) {
	query := `SELECT COUNT(*) FROM image_files`
	var count int
	err := db.conn.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get total image file count: %w", err)
	}
	return count, nil
}