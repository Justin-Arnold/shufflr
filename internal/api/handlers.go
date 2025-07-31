package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"shufflr/internal/auth"
	"shufflr/internal/models"
	"shufflr/internal/storage"
	"strconv"
	"strings"
)

type Server struct {
	db          *storage.DB
	authService *auth.AuthService
	uploadDir   string
}

func NewServer(db *storage.DB, authService *auth.AuthService, uploadDir string) *Server {
	return &Server{
		db:          db,
		authService: authService,
		uploadDir:   uploadDir,
	}
}

type RandomImagesResponse struct {
	Images []ImageResponse `json:"images"`
	Count  int             `json:"count"`
}

type ImageResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

func (s *Server) setCORSHeaders(w http.ResponseWriter) {
	// Get CORS settings
	corsEnabled, err := s.db.GetSetting("cors_enabled")
	if err != nil {
		corsEnabled = "true" // Default to enabled
	}

	if corsEnabled == "true" {
		corsOrigins, err := s.db.GetSetting("cors_origins")
		if err != nil || corsOrigins == "" {
			corsOrigins = "*"
		}
		
		w.Header().Set("Access-Control-Allow-Origin", corsOrigins)
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "X-API-Key, Authorization")
	}
}

func (s *Server) HandleRandomImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if API key is required
	requireAPIKey, err := s.db.GetSetting("require_api_key_for_images")
	if err != nil {
		log.Printf("Error getting API key requirement setting: %v", err)
		requireAPIKey = "true" // Default to secure
	}

	var apiKey *models.APIKey
	if requireAPIKey == "true" {
		// Get API key from context (set by middleware)
		apiKey = auth.GetAPIKeyFromContext(r.Context())
		if apiKey == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Get default image count from settings
	defaultCountStr, err := s.db.GetSetting("default_image_count")
	if err != nil || defaultCountStr == "" {
		defaultCountStr = "20"
	}
	defaultCount, _ := strconv.Atoi(defaultCountStr)

	// Get max image count from settings
	maxCountStr, err := s.db.GetSetting("max_image_count")
	if err != nil || maxCountStr == "" {
		maxCountStr = "100"
	}
	maxCount, _ := strconv.Atoi(maxCountStr)

	// Parse count parameter
	countStr := r.URL.Query().Get("count")
	count := defaultCount // Use setting default
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count <= 0 {
			http.Error(w, "Invalid count parameter", http.StatusBadRequest)
			return
		}
	}

	// Check if requested count exceeds maximum
	if count > maxCount {
		http.Error(w, fmt.Sprintf("Requested count (%d) exceeds maximum allowed (%d)", count, maxCount), http.StatusBadRequest)
		return
	}

	// Check if requested count exceeds total images
	totalImages, err := s.db.GetImageFileCount()
	if err != nil {
		log.Printf("Error getting image count: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if count > totalImages {
		http.Error(w, fmt.Sprintf("Requested count (%d) exceeds total images (%d)", count, totalImages), http.StatusBadRequest)
		return
	}

	// Get random images
	images, err := s.db.GetRandomImageFiles(count)
	if err != nil {
		log.Printf("Error getting random images: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	response := RandomImagesResponse{
		Images: make([]ImageResponse, len(images)),
		Count:  len(images),
	}

	for i, img := range images {
		response.Images[i] = ImageResponse{
			URL:      fmt.Sprintf("/api/images/%s", img.Filename),
			Filename: img.Filename,
		}
	}

	// Log API request if API key is used
	if apiKey != nil {
		if err := s.db.LogAPIRequest(apiKey.ID, len(images)); err != nil {
			log.Printf("Error logging API request: %v", err)
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	s.setCORSHeaders(w)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandleServeImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if API key is required for image access
	requireAPIKey, err := s.db.GetSetting("require_api_key_for_images")
	if err != nil {
		log.Printf("Error getting API key requirement setting: %v", err)
		requireAPIKey = "true" // Default to secure
	}

	if requireAPIKey == "true" {
		// Check for API key in headers when required
		apiKeyHeader := r.Header.Get("X-API-Key")
		if apiKeyHeader == "" {
			apiKeyHeader = r.Header.Get("Authorization")
			if strings.HasPrefix(apiKeyHeader, "Bearer ") {
				apiKeyHeader = strings.TrimPrefix(apiKeyHeader, "Bearer ")
			}
		}
		
		if apiKeyHeader == "" {
			http.Error(w, "API key required", http.StatusUnauthorized)
			return
		}

		// Validate API key
		apiKey, err := s.db.GetAPIKeyByKey(apiKeyHeader)
		if err != nil || apiKey == nil {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// Update last used timestamp
		if err := s.db.UpdateAPIKeyLastUsed(apiKey.ID); err != nil {
			log.Printf("Error updating API key last used: %v", err)
		}
	}

	// Extract filename from URL path
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/images/") {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	filename := strings.TrimPrefix(path, "/api/images/")
	if filename == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Security: prevent directory traversal
	filename = filepath.Base(filename)
	
	// Check if image exists in database and is enabled
	images, err := s.db.GetAllImageFiles()
	if err != nil {
		log.Printf("Error getting image files: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var foundImage bool
	var mimeType string
	var isEnabled bool
	for _, img := range images {
		if img.Filename == filename {
			foundImage = true
			mimeType = img.MimeType
			isEnabled = img.Enabled
			break
		}
	}

	if !foundImage || !isEnabled {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	// Serve the file
	filePath := filepath.Join(s.uploadDir, filename)
	
	// Check if file exists on filesystem
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Image file not found", http.StatusNotFound)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	s.setCORSHeaders(w)

	// Serve the file
	http.ServeFile(w, r, filePath)
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check database connection
	imageCount, err := s.db.GetImageFileCount()
	if err != nil {
		log.Printf("Health check failed - database error: %v", err)
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}

	response := map[string]interface{}{
		"status":      "healthy",
		"image_count": imageCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) HandleOptions(w http.ResponseWriter, r *http.Request) {
	// Get CORS settings
	corsEnabled, err := s.db.GetSetting("cors_enabled")
	if err != nil {
		corsEnabled = "true" // Default to enabled
	}

	if corsEnabled == "true" {
		corsOrigins, err := s.db.GetSetting("cors_origins")
		if err != nil || corsOrigins == "" {
			corsOrigins = "*"
		}
		
		w.Header().Set("Access-Control-Allow-Origin", corsOrigins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "X-API-Key, Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")
	}
	
	w.WriteHeader(http.StatusOK)
}