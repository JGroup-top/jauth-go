package jauth

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
)

// AuthorizationURL builds a login URL. The application must generate, store,
// and validate a fresh state for each login. Options support PKCE S256.
func (c *Client) AuthorizationURL(state string, opts ...oauth2.AuthCodeOption) string {
	return c.oauthConfig.AuthCodeURL(state, opts...)
}

// Exchange exchanges a code after the application has validated callback state.
// For PKCE, pass oauth2.VerifierOption with the original verifier.
func (c *Client) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	if strings.TrimSpace(code) == "" {
		return nil, ErrMissingCode
	}
	token, err := c.oauthConfig.Exchange(c.requestContext(ctx), code, opts...)
	if err != nil {
		return nil, fmt.Errorf("jauth: exchange code: %w", err)
	}
	return token, nil
}

// Authenticate exchanges a code and fetches its user. Use Exchange and UserInfo
// separately if the application needs to retain the token.
func (c *Client) Authenticate(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (User, error) {
	token, err := c.Exchange(ctx, code, opts...)
	if err != nil {
		return User{}, err
	}
	return c.UserInfo(ctx, token)
}
