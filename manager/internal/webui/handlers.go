package webui

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"manager/pkg/auth"
)

// LoginHandler handles the login page
type LoginHandler struct{}

// NewLoginHandler creates a new login handler
func NewLoginHandler() *LoginHandler {
	return &LoginHandler{}
}

// ServeHTTP handles requests to /login
func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Render the login page
	data := map[string]interface{}{
		"Title":       "Admin Login - BMC Manager",
		"HeaderTitle": "BMC Admin Login",
	}

	templates := GetLoginTemplates()
	if err := templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Error().Err(err).Msg("Failed to render login template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// LogoutHandler handles logout requests
type LogoutHandler struct{}

// NewLogoutHandler creates a new logout handler
func NewLogoutHandler() *LogoutHandler {
	return &LogoutHandler{}
}

// ServeHTTP handles requests to /logout
func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clear the auth_token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// AdminDashboardHandler handles the admin dashboard web UI
type AdminDashboardHandler struct {
	jwtManager *auth.JWTManager
}

// NewAdminDashboardHandler creates a new admin dashboard handler
func NewAdminDashboardHandler(jwtManager *auth.JWTManager) *AdminDashboardHandler {
	return &AdminDashboardHandler{
		jwtManager: jwtManager,
	}
}

// ServeHTTP handles requests to /admin
func (h *AdminDashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract JWT token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Try to get from cookie
		cookie, err := r.Cookie("auth_token")
		if err == nil && cookie.Value != "" {
			authHeader = "Bearer " + cookie.Value
		}
	}

	if authHeader == "" {
		http.Error(w, "Unauthorized: Missing authentication", http.StatusUnauthorized)
		return
	}

	// Extract token from "Bearer <token>"
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		http.Error(w, "Unauthorized: Invalid authorization format", http.StatusUnauthorized)
		return
	}

	// Validate token
	claims, err := h.jwtManager.ValidateToken(tokenString)
	if err != nil {
		log.Error().Err(err).Msg("Failed to validate token")
		http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
		return
	}

	// Check if user is admin
	if !claims.IsAdmin {
		log.Warn().Str("email", claims.Email).Msg("Non-admin user attempted to access admin dashboard")
		http.Error(w, "Forbidden: Admin privileges required", http.StatusForbidden)
		return
	}

	// Render the admin dashboard
	data := map[string]interface{}{
		"Title":       "Admin Dashboard - BMC Manager",
		"HeaderTitle": "BMC Admin Dashboard",
		"UserEmail":   claims.Email,
		"Token":       tokenString,
	}

	templates := GetAdminTemplates()
	if err := templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Error().Err(err).Msg("Failed to render admin dashboard template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
