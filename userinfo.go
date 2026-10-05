package jauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

const maxUserInfoBytes = 1 << 20

// UserInfo fetches the user, refreshing expired tokens if a refresh token exists.
// Refreshed tokens are scoped to this call and are not written back to token.
func (c *Client) UserInfo(ctx context.Context, token *oauth2.Token) (User, error) {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return User{}, ErrMissingToken
	}
	ctx = c.requestContext(ctx)
	current, err := c.oauthConfig.TokenSource(ctx, token).Token()
	if err != nil {
		return User{}, fmt.Errorf("jauth: refresh token: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.userInfoURL, nil)
	if err != nil {
		return User{}, fmt.Errorf("jauth: create userinfo request: %w", err)
	}
	current.SetAuthHeader(req)
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("jauth: userinfo request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return User{}, &HTTPError{StatusCode: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxUserInfoBytes+1))
	if err != nil {
		return User{}, fmt.Errorf("jauth: read userinfo: %w", err)
	}
	if len(data) > maxUserInfoBytes {
		return User{}, fmt.Errorf("%w: response exceeds 1 MiB", ErrInvalidUserInfo)
	}
	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return User{}, fmt.Errorf("%w: malformed JSON", ErrInvalidUserInfo)
	}
	if strings.TrimSpace(user.ID) == "" {
		return User{}, fmt.Errorf("%w: missing sub", ErrInvalidUserInfo)
	}
	return user, nil
}
