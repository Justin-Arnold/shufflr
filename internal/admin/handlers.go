package admin

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"shufflr/internal/auth"
	"shufflr/internal/models"
	"shufflr/internal/storage"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	db          *storage.DB
	authService *auth.AuthService
	uploadDir   string
	baseURL     string
}

func NewServer(db *storage.DB, authService *auth.AuthService, uploadDir, baseURL string) (*Server, error) {
	return &Server{
		db:          db,
		authService: authService,
		uploadDir:   uploadDir,
		baseURL:     baseURL,
	}, nil
}

type PageData struct {
	Title     string
	ShowNav   bool
	ActivePage string
	Username  string
	BaseURL   string
	Success   string
	Error     string
}

// Setup and login handlers
func (s *Server) HandleSetup(w http.ResponseWriter, r *http.Request) {
	hasAdmins, err := s.db.HasAdminUsers()
	if err != nil {
		log.Printf("Error checking admin users: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if hasAdmins {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}

	data := PageData{
		Title:   "Setup",
		ShowNav: false,
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if username == "" || password == "" {
			data.Error = "Username and password are required"
		} else if len(username) < 3 {
			data.Error = "Username must be at least 3 characters"
		} else if len(password) < 6 {
			data.Error = "Password must be at least 6 characters"
		} else if password != confirmPassword {
			data.Error = "Passwords do not match"
		} else {
			_, err := s.db.CreateAdminUser(username, password)
			if err != nil {
				log.Printf("Error creating admin user: %v", err)
				data.Error = "Failed to create admin user"
			} else {
				http.Redirect(w, r, "/admin/login?success=Admin account created successfully", http.StatusSeeOther)
				return
			}
		}
		data.Username = username
	}

	s.renderTemplate(w, "setup.html", data)
}

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:   "Login",
		ShowNav: false,
		Success: r.URL.Query().Get("success"),
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == "" || password == "" {
			data.Error = "Username and password are required"
		} else {
			user, err := s.authService.LoginAdmin(username, password)
			if err != nil {
				log.Printf("Error during login: %v", err)
				data.Error = "Login failed"
			} else if user == nil {
				data.Error = "Invalid username or password"
			} else {
				if err := s.authService.SetAdminSession(w, r, user); err != nil {
					log.Printf("Error setting session: %v", err)
					data.Error = "Login failed"
				} else {
					http.Redirect(w, r, "/admin", http.StatusSeeOther)
					return
				}
			}
		}
		data.Username = username
	}

	s.renderTemplate(w, "login.html", data)
}

func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.authService.ClearAdminSession(w, r); err != nil {
		log.Printf("Error clearing session: %v", err)
	}
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// Dashboard
func (s *Server) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.GetAdminFromContext(r.Context())
	
	imageCount, err := s.db.GetImageFileCount()
	if err != nil {
		log.Printf("Error getting image count: %v", err)
		imageCount = 0
	}

	apiKeys, err := s.db.GetAllAPIKeys()
	if err != nil {
		log.Printf("Error getting API keys: %v", err)
		apiKeys = []*models.APIKey{}
	}

	activeAPIKeyCount := 0
	for _, key := range apiKeys {
		if key.Enabled {
			activeAPIKeyCount++
		}
	}

	// Get total request count across all API keys
	requestCount := 0
	for _, key := range apiKeys {
		count, err := s.db.GetAPIKeyUsageCount(key.ID)
		if err == nil {
			requestCount += count
		}
	}

	data := struct {
		PageData
		ImageCount       int
		APIKeyCount      int
		ActiveAPIKeyCount int
		RequestCount     int
	}{
		PageData: PageData{
			Title:      "Dashboard",
			ShowNav:    true,
			ActivePage: "dashboard",
			Username:   user.Username,
			BaseURL:    s.baseURL,
			Success:    r.URL.Query().Get("success"),
		},
		ImageCount:        imageCount,
		APIKeyCount:       len(apiKeys),
		ActiveAPIKeyCount: activeAPIKeyCount,
		RequestCount:      requestCount,
	}

	s.renderTemplate(w, "dashboard.html", data)
}

// Image management
func (s *Server) HandleImages(w http.ResponseWriter, r *http.Request) {
	user := auth.GetAdminFromContext(r.Context())
	
	images, err := s.db.GetAllImageFiles()
	if err != nil {
		log.Printf("Error getting images: %v", err)
		images = []*models.ImageFile{}
	}

	// Add formatted data for display
	type ImageDisplay struct {
		*models.ImageFile
		SizeFormatted        string
		UploadedAtFormatted  string
	}

	displayImages := make([]ImageDisplay, len(images))
	var totalSize int64
	for i, img := range images {
		totalSize += img.Size
		displayImages[i] = ImageDisplay{
			ImageFile:           img,
			SizeFormatted:       formatFileSize(img.Size),
			UploadedAtFormatted: img.UploadedAt.Format("Jan 2, 2006"),
		}
	}

	data := struct {
		PageData
		Images              []ImageDisplay
		TotalSizeFormatted  string
	}{
		PageData: PageData{
			Title:      "Images",
			ShowNav:    true,
			ActivePage: "images",
			Username:   user.Username,
			Success:    r.URL.Query().Get("success"),
			Error:      r.URL.Query().Get("error"),
		},
		Images:             displayImages,
		TotalSizeFormatted: formatFileSize(totalSize),
	}

	s.renderTemplate(w, "images.html", data)
}

func (s *Server) HandleImageUpload(w http.ResponseWriter, r *http.Request) {
	user := auth.GetAdminFromContext(r.Context())
	
	if r.Method == http.MethodGet {
		data := PageData{
			Title:      "Upload Images",
			ShowNav:    true,
			ActivePage: "images",
			Username:   user.Username,
		}
		s.renderTemplate(w, "upload.html", data)
		return
	}

	// Handle POST - file upload
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		http.Redirect(w, r, "/admin/images/upload?error=No files selected", http.StatusSeeOther)
		return
	}

	var uploadedFiles []string
	var errors []string

	for _, fileHeader := range files {
		if err := s.uploadFile(fileHeader); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", fileHeader.Filename, err))
		} else {
			uploadedFiles = append(uploadedFiles, fileHeader.Filename)
		}
	}

	if len(errors) > 0 {
		errorMsg := "Some files failed to upload: " + strings.Join(errors, ", ")
		if len(uploadedFiles) > 0 {
			errorMsg += fmt.Sprintf(". %d files uploaded successfully.", len(uploadedFiles))
		}
		http.Redirect(w, r, "/admin/images?error="+errorMsg, http.StatusSeeOther)
	} else {
		successMsg := fmt.Sprintf("%d images uploaded successfully", len(uploadedFiles))
		http.Redirect(w, r, "/admin/images?success="+successMsg, http.StatusSeeOther)
	}
}

func (s *Server) uploadFile(fileHeader *multipart.FileHeader) error {
	// Validate file type
	if !isValidImageType(fileHeader.Header.Get("Content-Type")) {
		return fmt.Errorf("invalid file type")
	}

	// Open uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer file.Close()

	// Create unique filename if file already exists
	filename := fileHeader.Filename
	filePath := filepath.Join(s.uploadDir, filename)
	counter := 1
	for {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		// File exists, create new name
		ext := filepath.Ext(fileHeader.Filename)
		name := strings.TrimSuffix(fileHeader.Filename, ext)
		filename = fmt.Sprintf("%s_%d%s", name, counter, ext)
		filePath = filepath.Join(s.uploadDir, filename)
		counter++
	}

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy file data
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath) // Clean up on error
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	// Get file info
	fileInfo, err := dst.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Save to database
	_, err = s.db.CreateImageFile(filename, fileInfo.Size(), fileHeader.Header.Get("Content-Type"))
	if err != nil {
		os.Remove(filePath) // Clean up on error
		return fmt.Errorf("failed to save to database: %w", err)
	}

	return nil
}

func (s *Server) HandleImageRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	oldFilename := r.FormValue("old_filename")
	newFilename := r.FormValue("new_filename")

	if oldFilename == "" || newFilename == "" {
		http.Redirect(w, r, "/admin/images?error=Invalid filename", http.StatusSeeOther)
		return
	}

	// Validate new filename
	if !isValidFilename(newFilename) {
		http.Redirect(w, r, "/admin/images?error=Invalid filename format", http.StatusSeeOther)
		return
	}

	oldPath := filepath.Join(s.uploadDir, oldFilename)
	newPath := filepath.Join(s.uploadDir, newFilename)

	// Check if old file exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		http.Redirect(w, r, "/admin/images?error=Original file not found", http.StatusSeeOther)
		return
	}

	// Check if new filename already exists
	if _, err := os.Stat(newPath); err == nil {
		http.Redirect(w, r, "/admin/images?error=File with new name already exists", http.StatusSeeOther)
		return
	}

	// Rename file
	if err := os.Rename(oldPath, newPath); err != nil {
		log.Printf("Error renaming file: %v", err)
		http.Redirect(w, r, "/admin/images?error=Failed to rename file", http.StatusSeeOther)
		return
	}

	// Update database
	if err := s.db.UpdateImageFilename(oldFilename, newFilename); err != nil {
		log.Printf("Error updating database: %v", err)
		// Try to revert file rename
		os.Rename(newPath, oldPath)
		http.Redirect(w, r, "/admin/images?error=Failed to update database", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/images?success=Image renamed successfully", http.StatusSeeOther)
}

func (s *Server) HandleImageDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.FormValue("filename")
	if filename == "" {
		http.Redirect(w, r, "/admin/images?error=Invalid filename", http.StatusSeeOther)
		return
	}

	filePath := filepath.Join(s.uploadDir, filename)

	// Delete from database first
	if err := s.db.DeleteImageFile(filename); err != nil {
		log.Printf("Error deleting from database: %v", err)
		http.Redirect(w, r, "/admin/images?error=Failed to delete from database", http.StatusSeeOther)
		return
	}

	// Delete file from filesystem
	if err := os.Remove(filePath); err != nil {
		log.Printf("Error deleting file: %v", err)
		// File deletion failed, but database was updated - this is a partial failure
		// Could recreate DB entry here, but for simplicity we'll just log it
	}

	http.Redirect(w, r, "/admin/images?success=Image deleted successfully", http.StatusSeeOther)
}

// Helper functions
func (s *Server) renderTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Parse base template and the specific page template
	tmpl := template.New("").Funcs(template.FuncMap{
		"formatFileSize": formatFileSize,
		"formatTime": func(t time.Time) string {
			return t.Format("Jan 2, 2006 3:04 PM")
		},
	})
	
	// Parse base template first
	tmpl, err := tmpl.ParseFiles("web/templates/base.html", "web/templates/"+templateName)
	if err != nil {
		log.Printf("Template parsing error for %s: %v", templateName, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	// Execute the base template
	if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Template execution error for %s: %v", templateName, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func isValidImageType(contentType string) bool {
	validTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
	}
	
	for _, validType := range validTypes {
		if contentType == validType {
			return true
		}
	}
	return false
}

func isValidFilename(filename string) bool {
	// Basic filename validation
	if len(filename) == 0 || len(filename) > 255 {
		return false
	}
	
	// Check for invalid characters
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		if strings.Contains(filename, char) {
			return false
		}
	}
	
	return true
}

// API Key management
func (s *Server) HandleAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := auth.GetAdminFromContext(r.Context())
	
	apiKeys, err := s.db.GetAllAPIKeys()
	if err != nil {
		log.Printf("Error getting API keys: %v", err)
		apiKeys = []*models.APIKey{}
	}

	// Add formatted data and usage counts
	type APIKeyDisplay struct {
		*models.APIKey
		CreatedAtFormatted string
		LastUsedFormatted  string
		RequestCount       int
	}

	displayKeys := make([]APIKeyDisplay, len(apiKeys))
	for i, key := range apiKeys {
		requestCount, err := s.db.GetAPIKeyUsageCount(key.ID)
		if err != nil {
			log.Printf("Error getting usage count for key %d: %v", key.ID, err)
			requestCount = 0
		}

		lastUsedFormatted := "Never"
		if key.LastUsed != nil {
			lastUsedFormatted = key.LastUsed.Format("Jan 2, 2006 3:04 PM")
		}

		displayKeys[i] = APIKeyDisplay{
			APIKey:             key,
			CreatedAtFormatted: key.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
			LastUsedFormatted:  lastUsedFormatted,
			RequestCount:       requestCount,
		}
	}

	data := struct {
		PageData
		APIKeys []APIKeyDisplay
	}{
		PageData: PageData{
			Title:      "API Keys",
			ShowNav:    true,
			ActivePage: "api-keys",
			Username:   user.Username,
			Success:    r.URL.Query().Get("success"),
			Error:      r.URL.Query().Get("error"),
		},
		APIKeys: displayKeys,
	}

	s.renderTemplate(w, "api-keys.html", data)
}

func (s *Server) HandleNewAPIKey(w http.ResponseWriter, r *http.Request) {
	user := auth.GetAdminFromContext(r.Context())
	
	data := struct {
		PageData
		Name      string
		NewAPIKey string
	}{
		PageData: PageData{
			Title:      "Create API Key",
			ShowNav:    true,
			ActivePage: "api-keys",
			Username:   user.Username,
			BaseURL:    s.baseURL,
		},
	}

	if r.Method == http.MethodPost {
		name := strings.TrimSpace(r.FormValue("name"))
		
		if name == "" {
			data.Error = "API key name is required"
		} else if len(name) > 100 {
			data.Error = "API key name must be 100 characters or less"
		} else {
			apiKey, rawKey, err := s.db.CreateAPIKey(name)
			if err != nil {
				log.Printf("Error creating API key: %v", err)
				data.Error = "Failed to create API key"
			} else {
				log.Printf("Created API key: %s (ID: %d)", apiKey.Name, apiKey.ID)
				data.NewAPIKey = rawKey
				s.renderTemplate(w, "new-api-key.html", data)
				return
			}
		}
		data.Name = name
	}

	s.renderTemplate(w, "new-api-key.html", data)
}

func (s *Server) HandleToggleAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keyIDStr := r.FormValue("key_id")
	enabledStr := r.FormValue("enabled")

	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil {
		http.Redirect(w, r, "/admin/api-keys?error=Invalid key ID", http.StatusSeeOther)
		return
	}

	enabled := enabledStr == "true"

	if err := s.db.UpdateAPIKeyEnabled(keyID, enabled); err != nil {
		log.Printf("Error updating API key enabled status: %v", err)
		http.Redirect(w, r, "/admin/api-keys?error=Failed to update API key", http.StatusSeeOther)
		return
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/api-keys?success=API key %s successfully", action), http.StatusSeeOther)
}

func (s *Server) HandleRegenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keyIDStr := r.FormValue("key_id")
	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil {
		http.Redirect(w, r, "/admin/api-keys?error=Invalid key ID", http.StatusSeeOther)
		return
	}

	// Get existing key info
	keys, err := s.db.GetAllAPIKeys()
	if err != nil {
		log.Printf("Error getting API keys: %v", err)
		http.Redirect(w, r, "/admin/api-keys?error=Failed to regenerate API key", http.StatusSeeOther)
		return
	}

	var existingKey *models.APIKey
	for _, key := range keys {
		if key.ID == keyID {
			existingKey = key
			break
		}
	}

	if existingKey == nil {
		http.Redirect(w, r, "/admin/api-keys?error=API key not found", http.StatusSeeOther)
		return
	}

	// Delete old key and create new one with same name
	if err := s.db.DeleteAPIKey(keyID); err != nil {
		log.Printf("Error deleting old API key: %v", err)
		http.Redirect(w, r, "/admin/api-keys?error=Failed to regenerate API key", http.StatusSeeOther)
		return
	}

	newKey, _, err := s.db.CreateAPIKey(existingKey.Name)
	if err != nil {
		log.Printf("Error creating new API key: %v", err)
		http.Redirect(w, r, "/admin/api-keys?error=Failed to regenerate API key", http.StatusSeeOther)
		return
	}

	log.Printf("Regenerated API key: %s (old ID: %d, new ID: %d)", newKey.Name, keyID, newKey.ID)
	http.Redirect(w, r, "/admin/api-keys?success=API key regenerated successfully", http.StatusSeeOther)
}

func (s *Server) HandleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keyIDStr := r.FormValue("key_id")
	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil {
		http.Redirect(w, r, "/admin/api-keys?error=Invalid key ID", http.StatusSeeOther)
		return
	}

	if err := s.db.DeleteAPIKey(keyID); err != nil {
		log.Printf("Error deleting API key: %v", err)
		http.Redirect(w, r, "/admin/api-keys?error=Failed to delete API key", http.StatusSeeOther)
		return
	}

	log.Printf("Deleted API key ID: %d", keyID)
	http.Redirect(w, r, "/admin/api-keys?success=API key deleted successfully", http.StatusSeeOther)
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}