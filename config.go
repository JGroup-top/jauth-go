package jauth

import "net/http"

// DefaultBaseURL is the default JAuth server.
const DefaultBaseURL = "https://sso.jgroup.top"

// Config configures a JAuth client. The zero BaseURL uses DefaultBaseURL.
type Config struct {
	BaseURL  string
	ClientID string
	// ClientSecret may be empty for public clients using PKCE.
	ClientSecret string
	RedirectURL  string
	// Empty Scopes defaults to openid, profile, email.
	Scopes []string
	// HTTPClient is optional. The default has a 15-second timeout.
	// The client is copied; redirects are disabled to protect credentials.
	HTTPClient *http.Client
}
