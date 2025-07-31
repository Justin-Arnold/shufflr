package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"shufflr/internal/admin"
	"shufflr/internal/api"
	"shufflr/internal/auth"
	"shufflr/internal/storage"
	"strconv"
)

type Config struct {
	Port          string
	DatabasePath  string
	UploadDir     string
	SessionSecret string
	BaseURL       string
}

func main() {
	config := loadConfig()

	// Ensure upload directory exists
	if err := os.MkdirAll(config.UploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	// Initialize database
	db, err := storage.NewDB(config.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize auth service
	authService := auth.NewAuthService(db, config.SessionSecret)

	// Initialize servers
	adminServer, err := admin.NewServer(db, authService, config.UploadDir, config.BaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize admin server: %v", err)
	}

	apiServer := api.NewServer(db, authService, config.UploadDir)

	// Setup routes
	mux := http.NewServeMux()

	// Static files (favicon, etc.)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	// Health check
	mux.HandleFunc("/health", apiServer.HandleHealth)

	// API routes
	mux.HandleFunc("/api/images", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			apiServer.HandleOptions(w, r)
			return
		}
		// Get random images (conditionally requires API key based on settings)
		requireAPIKey, err := db.GetSetting("require_api_key_for_images")
		if err != nil || requireAPIKey == "true" {
			authService.RequireAPIKey(apiServer.HandleRandomImages)(w, r)
		} else {
			apiServer.HandleRandomImages(w, r)
		}
	})
	
	mux.HandleFunc("/api/images/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			apiServer.HandleOptions(w, r)
			return
		}
		// Serve individual image (API key requirement handled within the handler)
		apiServer.HandleServeImage(w, r)
	})

	// Admin routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
		http.NotFound(w, r)
	})

	// Check if admin setup is needed
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		hasAdmins, err := db.HasAdminUsers()
		if err != nil {
			log.Printf("Error checking admin users: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !hasAdmins {
			http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
			return
		}
		authService.RequireAdminAuth(adminServer.HandleDashboard)(w, r)
	})

	mux.HandleFunc("/admin/setup", adminServer.HandleSetup)
	mux.HandleFunc("/admin/login", adminServer.HandleLogin)
	mux.HandleFunc("/admin/logout", adminServer.HandleLogout)

	// Protected admin routes
	mux.HandleFunc("/admin/images", authService.RequireAdminAuth(adminServer.HandleImages))
	mux.HandleFunc("/admin/images/serve/", authService.RequireAdminAuth(adminServer.HandleServeImage))
	mux.HandleFunc("/admin/images/upload", authService.RequireAdminAuth(adminServer.HandleImageUpload))
	mux.HandleFunc("/admin/images/rename", authService.RequireAdminAuth(adminServer.HandleImageRename))
	mux.HandleFunc("/admin/images/delete", authService.RequireAdminAuth(adminServer.HandleImageDelete))
	mux.HandleFunc("/admin/images/toggle", authService.RequireAdminAuth(adminServer.HandleToggleImage))

	mux.HandleFunc("/admin/api-keys", authService.RequireAdminAuth(adminServer.HandleAPIKeys))
	mux.HandleFunc("/admin/api-keys/new", authService.RequireAdminAuth(adminServer.HandleNewAPIKey))
	mux.HandleFunc("/admin/api-keys/toggle", authService.RequireAdminAuth(adminServer.HandleToggleAPIKey))
	mux.HandleFunc("/admin/api-keys/regenerate", authService.RequireAdminAuth(adminServer.HandleRegenerateAPIKey))
	mux.HandleFunc("/admin/api-keys/delete", authService.RequireAdminAuth(adminServer.HandleDeleteAPIKey))

	mux.HandleFunc("/admin/settings", authService.RequireAdminAuth(adminServer.HandleSettings))

	// Add request logging middleware
	handler := loggingMiddleware(mux)

	log.Printf("Starting Shufflr server on port %s", config.Port)
	log.Printf("Upload directory: %s", config.UploadDir)
	log.Printf("Database: %s", config.DatabasePath)
	log.Printf("Base URL: %s", config.BaseURL)

	if err := http.ListenAndServe(":"+config.Port, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func loadConfig() Config {
	config := Config{
		Port:         getEnv("PORT", "8080"),
		DatabasePath: getEnv("DATABASE_PATH", "./shufflr.db"),
		UploadDir:    getEnv("UPLOAD_DIR", "./uploads"),
		BaseURL:      getEnv("BASE_URL", "http://localhost:8080"),
	}

	// Generate or load session secret
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		// Generate a random session secret
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			log.Fatalf("Failed to generate session secret: %v", err)
		}
		sessionSecret = hex.EncodeToString(bytes)
		log.Println("Generated new session secret. Set SESSION_SECRET environment variable to persist sessions across restarts.")
	}
	config.SessionSecret = sessionSecret

	// Validate upload directory path
	if !filepath.IsAbs(config.UploadDir) {
		absPath, err := filepath.Abs(config.UploadDir)
		if err != nil {
			log.Fatalf("Failed to resolve upload directory path: %v", err)
		}
		config.UploadDir = absPath
	}

	// Validate database path
	if !filepath.IsAbs(config.DatabasePath) {
		absPath, err := filepath.Abs(config.DatabasePath)
		if err != nil {
			log.Fatalf("Failed to resolve database path: %v", err)
		}
		config.DatabasePath = absPath
	}

	// Validate port
	if port, err := strconv.Atoi(config.Port); err != nil || port < 1 || port > 65535 {
		log.Fatalf("Invalid port: %s", config.Port)
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip logging for health checks to reduce noise
		if r.URL.Path != "/health" {
			log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		}
		next.ServeHTTP(w, r)
	})
}