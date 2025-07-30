package auth

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"shufflr/internal/models"
	"shufflr/internal/storage"
	"strings"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const (
	sessionName        = "shufflr-session"
	adminUserKey       = contextKey("admin_user")
	apiKeyKey          = contextKey("api_key")
	sessionUserIDKey   = "user_id"
	sessionUsernameKey = "username"
)

type AuthService struct {
	db    *storage.DB
	store sessions.Store
}

func NewAuthService(db *storage.DB, sessionSecret string) *AuthService {
	// Decode hex string to bytes for proper session encryption
	var keyBytes []byte
	var err error
	
	if len(sessionSecret)%2 == 0 && len(sessionSecret) >= 32 {
		// Try to decode as hex string first
		keyBytes, err = hex.DecodeString(sessionSecret)
		if err != nil {
			// If hex decode fails, use the string directly but truncate/pad to 32 bytes
			keyBytes = []byte(sessionSecret)
		}
	} else {
		// Use the string directly
		keyBytes = []byte(sessionSecret)
	}
	
	// Ensure key is exactly 32 bytes (required for AES-256)
	if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	} else if len(keyBytes) < 32 {
		// Pad with zeros if too short
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	}
	
	store := sessions.NewCookieStore(keyBytes)
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   24 * 60 * 60, // 24 hours
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}

	return &AuthService{
		db:    db,
		store: store,
	}
}

// Admin authentication
func (a *AuthService) LoginAdmin(username, password string) (*models.AdminUser, error) {
	user, err := a.db.GetAdminUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil // User not found
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, nil // Invalid password
	}

	return user, nil
}

func (a *AuthService) SetAdminSession(w http.ResponseWriter, r *http.Request, user *models.AdminUser) error {
	session, err := a.store.Get(r, sessionName)
	if err != nil {
		return err
	}

	session.Values[sessionUserIDKey] = user.ID
	session.Values[sessionUsernameKey] = user.Username

	return session.Save(r, w)
}

func (a *AuthService) ClearAdminSession(w http.ResponseWriter, r *http.Request) error {
	session, err := a.store.Get(r, sessionName)
	if err != nil {
		return err
	}

	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1

	return session.Save(r, w)
}

func (a *AuthService) GetAdminFromSession(r *http.Request) (*models.AdminUser, error) {
	session, err := a.store.Get(r, sessionName)
	if err != nil {
		return nil, err
	}

	userID, ok := session.Values[sessionUserIDKey].(int)
	if !ok {
		return nil, nil
	}

	username, ok := session.Values[sessionUsernameKey].(string)
	if !ok {
		return nil, nil
	}

	return &models.AdminUser{
		ID:       userID,
		Username: username,
	}, nil
}

// API key authentication
func (a *AuthService) ValidateAPIKey(apiKey string) (*models.APIKey, error) {
	return a.db.GetAPIKeyByKey(apiKey)
}

// Middleware
func (a *AuthService) RequireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := a.GetAdminFromSession(r)
		if err != nil {
			log.Printf("Error checking admin session: %v", err)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		if user == nil {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), adminUserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (a *AuthService) RequireAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check X-API-Key header first
		apiKey := r.Header.Get("X-API-Key")
		
		// If not found, check Authorization header
		if apiKey == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if apiKey == "" {
			http.Error(w, "API key required", http.StatusUnauthorized)
			return
		}

		key, err := a.ValidateAPIKey(apiKey)
		if err != nil {
			log.Printf("Error validating API key: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if key == nil {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// Update last used timestamp
		if err := a.db.UpdateAPIKeyLastUsed(key.ID); err != nil {
			log.Printf("Error updating API key last used: %v", err)
		}

		ctx := context.WithValue(r.Context(), apiKeyKey, key)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// Context helpers
func GetAdminFromContext(ctx context.Context) *models.AdminUser {
	user, ok := ctx.Value(adminUserKey).(*models.AdminUser)
	if !ok {
		return nil
	}
	return user
}

func GetAPIKeyFromContext(ctx context.Context) *models.APIKey {
	key, ok := ctx.Value(apiKeyKey).(*models.APIKey)
	if !ok {
		return nil
	}
	return key
}