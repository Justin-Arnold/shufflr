package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"shufflr/internal/auth"
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

func (s *Server) HandleRandomImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get API key from context (set by middleware)
	apiKey := auth.GetAPIKeyFromContext(r.Context())
	if apiKey == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse count parameter
	countStr := r.URL.Query().Get("count")
	count := 20 // default
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count <= 0 {
			http.Error(w, "Invalid count parameter", http.StatusBadRequest)
			return
		}
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

	// Log API request
	if err := s.db.LogAPIRequest(apiKey.ID, len(images)); err != nil {
		log.Printf("Error logging API request: %v", err)
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "X-API-Key, Authorization")

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
	w.Header().Set("Access-Control-Allow-Origin", "*")

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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "X-API-Key, Authorization, Content-Type")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusOK)
}