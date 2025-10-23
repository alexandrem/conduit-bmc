package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractJWTFromAuthHeader(t *testing.T) {
	tests := []struct {
		name        string
		authHeader  string
		wantToken   string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			wantToken:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			wantErr:    false,
		},
		{
			name:        "empty header",
			authHeader:  "",
			wantToken:   "",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "missing Bearer prefix",
			authHeader:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			wantToken:   "",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "wrong prefix",
			authHeader:  "Token abc123",
			wantToken:   "",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "bearer lowercase",
			authHeader:  "bearer abc123",
			wantToken:   "",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:       "bearer with extra spaces",
			authHeader: "Bearer  token-with-spaces",
			wantToken:  " token-with-spaces",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := ExtractJWTFromAuthHeader(tt.authHeader)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantToken, token)
			}
		})
	}
}

func TestExtractJWTFromCookie(t *testing.T) {
	tests := []struct {
		name         string
		cookieHeader string
		cookieName   string
		want         string
	}{
		{
			name:         "single cookie",
			cookieHeader: "auth_token=abc123",
			cookieName:   "auth_token",
			want:         "abc123",
		},
		{
			name:         "multiple cookies",
			cookieHeader: "session_id=xyz; auth_token=abc123; other=value",
			cookieName:   "auth_token",
			want:         "abc123",
		},
		{
			name:         "cookie with spaces",
			cookieHeader: "auth_token=abc123 ; other=value",
			cookieName:   "auth_token",
			want:         "abc123",
		},
		{
			name:         "cookie not found",
			cookieHeader: "session_id=xyz; other=value",
			cookieName:   "auth_token",
			want:         "",
		},
		{
			name:         "empty cookie header",
			cookieHeader: "",
			cookieName:   "auth_token",
			want:         "",
		},
		{
			name:         "empty cookie name",
			cookieHeader: "auth_token=abc123",
			cookieName:   "",
			want:         "",
		},
		{
			name:         "jwt token value",
			cookieHeader: "auth_token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			cookieName:   "auth_token",
			want:         "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		},
		{
			name:         "similar cookie name",
			cookieHeader: "my_auth_token=wrong; auth_token=correct",
			cookieName:   "auth_token",
			want:         "correct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractJWTFromCookie(tt.cookieHeader, tt.cookieName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractJWTFromRequest(t *testing.T) {
	tests := []struct {
		name         string
		authHeader   string
		cookieHeader string
		cookieName   string
		wantToken    string
		wantErr      bool
	}{
		{
			name:         "auth header takes precedence",
			authHeader:   "Bearer header-token",
			cookieHeader: "auth_token=cookie-token",
			cookieName:   "auth_token",
			wantToken:    "header-token",
			wantErr:      false,
		},
		{
			name:         "fallback to cookie when header empty",
			authHeader:   "",
			cookieHeader: "auth_token=cookie-token",
			cookieName:   "auth_token",
			wantToken:    "cookie-token",
			wantErr:      false,
		},
		{
			name:         "fallback to cookie when header invalid",
			authHeader:   "InvalidFormat",
			cookieHeader: "auth_token=cookie-token",
			cookieName:   "auth_token",
			wantToken:    "cookie-token",
			wantErr:      false,
		},
		{
			name:         "neither header nor cookie",
			authHeader:   "",
			cookieHeader: "",
			cookieName:   "auth_token",
			wantToken:    "",
			wantErr:      true,
		},
		{
			name:         "header present but cookie not found",
			authHeader:   "Bearer valid-token",
			cookieHeader: "other_cookie=value",
			cookieName:   "auth_token",
			wantToken:    "valid-token",
			wantErr:      false,
		},
		{
			name:         "only cookie present",
			authHeader:   "",
			cookieHeader: "session=xyz; auth_token=my-token; other=abc",
			cookieName:   "auth_token",
			wantToken:    "my-token",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := ExtractJWTFromRequest(tt.authHeader, tt.cookieHeader, tt.cookieName)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantToken, token)
			}
		})
	}
}
