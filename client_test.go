package jauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func testConfig() Config {
	return Config{ClientID: "app", ClientSecret: "secret", RedirectURL: "https://app.example/callback"}
}
func TestConfig(t *testing.T) {
	cfg := testConfig()
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(c.AuthorizationURL("random-state"))
	if u.Scheme+"://"+u.Host != DefaultBaseURL || u.Path != "/oauth2/authorize" || u.Query().Get("state") != "random-state" || u.Query().Get("scope") != "openid profile email" || u.Query().Get("response_type") != "code" {
		t.Fatal(u)
	}
	cfg.BaseURL = "http://localhost:3001/prefix///"
	cfg.Scopes = []string{"profile"}
	c, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Scopes[0] = "changed"
	u, _ = url.Parse(c.AuthorizationURL("state"))
	if u.Path != "/prefix/oauth2/authorize" || u.Query().Get("scope") != "profile" || c.userInfoURL != "http://localhost:3001/prefix/userinfo" {
		t.Fatal(u, c.userInfoURL)
	}
	for _, base := range []string{"relative", "ftp://host", "https://host?x=1", "https://host#fragment", "https://user:pass@host"} {
		cfg := testConfig()
		cfg.BaseURL = base
		if _, err := New(cfg); !errors.Is(err, ErrInvalidBaseURL) {
			t.Fatalf("%q: %v", base, err)
		}
	}
	for _, tc := range []struct {
		change func(*Config)
		want   error
	}{
		{func(c *Config) { c.ClientID = " " }, ErrMissingClientID},
		{func(c *Config) { c.RedirectURL = "" }, ErrMissingRedirectURL},
		{func(c *Config) { c.RedirectURL = "/callback" }, ErrInvalidRedirectURL},
	} {
		cfg := testConfig()
		tc.change(&cfg)
		if _, err := New(cfg); !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
	}
}

func TestAuthenticatePKCEAndRefresh(t *testing.T) {
	verifier := oauth2.GenerateVerifier()
	tokenCalls, userCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2/token":
			tokenCalls++
			if r.Method != "POST" {
				t.Error(r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			id, secret, ok := r.BasicAuth()
			if !ok || id != "app" || secret != "secret" {
				t.Error("client authentication missing")
			}
			if r.Form.Get("grant_type") == "authorization_code" {
				if r.Form.Get("code") != "code" || r.Form.Get("code_verifier") != verifier || r.Form.Get("redirect_uri") != testConfig().RedirectURL {
					t.Error(r.Form)
				}
			} else if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh" {
				t.Error(r.Form)
			}
			w.Write([]byte(`{"access_token":"access","token_type":"Bearer","expires_in":3600,"refresh_token":"refresh"}`))
		case "/userinfo":
			userCalls++
			if r.Header.Get("Authorization") != "Bearer access" {
				t.Error("missing bearer token")
			}
			w.Write([]byte(`{"sub":"42","preferred_username":"alex","email":"a@example.com","extra":true}`))
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	cfg := testConfig()
	cfg.BaseURL = server.URL + "/"
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(c.AuthorizationURL("state", oauth2.S256ChallengeOption(verifier)))
	if u.Query().Get("code_challenge") != oauth2.S256ChallengeFromVerifier(verifier) || u.Query().Get("code_challenge_method") != "S256" {
		t.Fatal(u)
	}
	user, err := c.Authenticate(context.Background(), "code", oauth2.VerifierOption(verifier))
	if err != nil || user.ID != "42" || user.Username != "alex" {
		t.Fatal(user, err)
	}
	expired := &oauth2.Token{AccessToken: "old", RefreshToken: "refresh", Expiry: time.Now().Add(-time.Hour)}
	if _, err := c.UserInfo(context.Background(), expired); err != nil {
		t.Fatal(err)
	}
	if tokenCalls != 2 || userCalls != 2 || expired.AccessToken != "old" {
		t.Fatal(tokenCalls, userCalls, expired)
	}
}

func TestUserInfoFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       error
	}{
		{"unauthorized", "private body", 401, nil}, {"redirect", "", 302, nil},
		{"malformed", "{", 200, ErrInvalidUserInfo}, {"empty subject", `{"sub":""}`, 200, ErrInvalidUserInfo},
		{"null", "null", 200, ErrInvalidUserInfo}, {"trailing", `{"sub":"1"} {}`, 200, ErrInvalidUserInfo},
		{"too large", strings.Repeat("x", maxUserInfoBytes+1), 200, ErrInvalidUserInfo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "/should-not-follow")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer server.Close()
			cfg := testConfig()
			cfg.BaseURL = server.URL
			c, _ := New(cfg)
			_, err := c.UserInfo(context.Background(), &oauth2.Token{AccessToken: "token"})
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatal(err)
				}
			} else {
				var he *HTTPError
				if !errors.As(err, &he) || he.StatusCode != tc.status {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestExchangeFailureAndCancellation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()
	cfg := testConfig()
	cfg.BaseURL = server.URL
	c, _ := New(cfg)
	if _, err := c.Exchange(context.Background(), ""); !errors.Is(err, ErrMissingCode) {
		t.Fatal(err)
	}
	if _, err := c.UserInfo(context.Background(), nil); !errors.Is(err, ErrMissingToken) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("unexpected request")
	}
	_, err := c.Authenticate(context.Background(), "bad")
	var re *oauth2.RetrieveError
	if !errors.As(err, &re) || re.ErrorCode != "invalid_grant" {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Exchange(ctx, "code"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := c.UserInfo(ctx, &oauth2.Token{AccessToken: "token"}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
