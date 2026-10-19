// Package jauth provides a small OAuth2 client for JAuth.
package jauth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Client can be reused concurrently. Construct it with New.
type Client struct {
	oauthConfig *oauth2.Config
	userInfoURL string
	httpClient  *http.Client
}

// New validates configuration without making network requests.
func New(config Config) (*Client, error) {
	if strings.TrimSpace(config.ClientID) == "" {
		return nil, ErrMissingClientID
	}
	if strings.TrimSpace(config.RedirectURL) == "" {
		return nil, ErrMissingRedirectURL
	}
	redirect, err := url.Parse(config.RedirectURL)
	if err != nil || !redirect.IsAbs() || redirect.Fragment != "" || redirect.User != nil || ((redirect.Scheme == "http" || redirect.Scheme == "https") && redirect.Hostname() == "") {
		return nil, ErrInvalidRedirectURL
	}
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	base, err := url.Parse(config.BaseURL)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" {
		return nil, ErrInvalidBaseURL
	}
	config.BaseURL = strings.TrimRight(base.String(), "/")
	scopes := append([]string(nil), config.Scopes...)
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}
	hc := &http.Client{Timeout: 15 * time.Second}
	if config.HTTPClient != nil {
		copy := *config.HTTPClient
		hc = &copy
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{
		httpClient:  hc,
		userInfoURL: config.BaseURL + "/userinfo",
		oauthConfig: &oauth2.Config{
			ClientID: config.ClientID, ClientSecret: config.ClientSecret,
			RedirectURL: config.RedirectURL, Scopes: scopes,
			Endpoint: oauth2.Endpoint{AuthURL: config.BaseURL + "/oauth2/authorize", TokenURL: config.BaseURL + "/oauth2/token"},
		},
	}, nil
}

func (c *Client) requestContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
}
