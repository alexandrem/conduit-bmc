package api

// CreateProxyRequest represents a request to create a proxy session
type CreateProxyRequest struct {
	ServerID string `json:"server_id"`
}

// CreateProxyResponse represents the response from creating a proxy session
type CreateProxyResponse struct {
	SessionID string `json:"session_id"`
	Endpoint  string `json:"endpoint"`
	ExpiresAt string `json:"expires_at"`
}
