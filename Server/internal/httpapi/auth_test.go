package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/G6kco/CyberSpace/internal/app"
	"github.com/G6kco/CyberSpace/internal/auth"
	"github.com/G6kco/CyberSpace/internal/auth/authtest"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	testFrontendURL   = "http://localhost:5173"
	testAllowedDomain = "bitsathy.ac.in"
)

type authServer struct {
	t      *testing.T
	google *authtest.Google
	repo   *authtest.MemoryRepository
	user   *authtest.UserRecord
	router *gin.Engine
	logs   *observer.ObservedLogs
	secure bool
	// application lets a test add dependencies and rebuild the router.
	application *app.App
}

func newAuthServer(t *testing.T, appEnv string) *authServer {
	t.Helper()
	gin.SetMode(gin.TestMode)

	google := authtest.NewGoogle(t)
	repo := authtest.NewMemoryRepository()
	login, err := auth.NewGoogleLogin(
		google.Context(context.Background()),
		authtest.Config(testAllowedDomain),
		repo,
	)
	if err != nil {
		t.Fatalf("NewGoogleLogin() error = %v", err)
	}

	core, logs := observer.New(zap.DebugLevel)
	cfg := authtest.Config(testAllowedDomain)
	cfg.AppEnv = appEnv
	cfg.FrontendURL = testFrontendURL
	cfg.AllowedOrigins = []string{testFrontendURL}

	application, err := app.New(cfg, fakeDatabase{}, zap.New(core))
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	application.Auth = login

	return &authServer{
		t:      t,
		google: google,
		repo:   repo,
		user:   repo.AddUser("student@bitsathy.ac.in", "student", "active"),
		router: NewRouter(application),
		logs:   logs,
		secure: appEnv == "production",

		application: application,
	}
}

func (s *authServer) account() authtest.Account {
	return authtest.Account{
		Subject:       "google-subject-1",
		Email:         s.user.Email,
		EmailVerified: true,
		HostedDomain:  testAllowedDomain,
	}
}

func (s *authServer) cookieName(kind string) string {
	if s.secure {
		return "__Host-cyberspace_" + kind
	}
	return "cyberspace_" + kind
}

// do sends a request through the router. The context carries the fake
// Google's HTTP client, which the callback's code exchange uses.
func (s *authServer) do(method, target string, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	s.t.Helper()

	request := httptest.NewRequest(method, target, nil)
	request = request.WithContext(s.google.Context(request.Context()))
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

func responseCookie(recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// start begins a login and returns the login cookie and Google's callback
// query for account.
func (s *authServer) start(account authtest.Account) (*http.Cookie, url.Values) {
	s.t.Helper()

	response := s.do(http.MethodGet, "/api/v1/auth/google", nil, nil)
	if response.Code != http.StatusFound {
		s.t.Fatalf("start status = %d, want %d", response.Code, http.StatusFound)
	}
	loginCookie := responseCookie(response, s.cookieName("login"))
	if loginCookie == nil {
		s.t.Fatal("start did not set the login cookie")
	}

	state, code := s.google.Authorize(response.Header().Get("Location"), account)
	return loginCookie, url.Values{"state": {state}, "code": {code}}
}

func (s *authServer) callback(loginCookie *http.Cookie, query url.Values) *httptest.ResponseRecorder {
	s.t.Helper()

	var cookies []*http.Cookie
	if loginCookie != nil {
		cookies = append(cookies, loginCookie)
	}
	return s.do(http.MethodGet, "/api/v1/auth/google/callback?"+query.Encode(), cookies, nil)
}

func (s *authServer) signIn() *http.Cookie {
	s.t.Helper()

	response := s.callback(s.start(s.account()))
	if response.Code != http.StatusSeeOther {
		s.t.Fatalf("callback status = %d, want %d; body %s", response.Code, http.StatusSeeOther, response.Body)
	}
	session := responseCookie(response, s.cookieName("session"))
	if session == nil {
		s.t.Fatal("callback did not set the session cookie")
	}
	return session
}

func (s *authServer) me(session *http.Cookie) *httptest.ResponseRecorder {
	var cookies []*http.Cookie
	if session != nil {
		cookies = append(cookies, session)
	}
	return s.do(http.MethodGet, "/api/v1/me", cookies, nil)
}

func (s *authServer) logout(session *http.Cookie, origin string) *httptest.ResponseRecorder {
	var cookies []*http.Cookie
	if session != nil {
		cookies = append(cookies, session)
	}
	headers := map[string]string{}
	if origin != "" {
		headers["Origin"] = origin
	}
	return s.do(http.MethodDelete, "/api/v1/auth/session", cookies, headers)
}

func errorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", recorder.Body, err)
	}
	return body.Code
}

func assertCookieAttributes(t *testing.T, cookie *http.Cookie, secure bool, maxAge int) {
	t.Helper()

	if !cookie.HttpOnly {
		t.Errorf("cookie %s is not HttpOnly", cookie.Name)
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie %s SameSite = %v, want Lax", cookie.Name, cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Errorf("cookie %s Path = %q, want /", cookie.Name, cookie.Path)
	}
	if cookie.Secure != secure {
		t.Errorf("cookie %s Secure = %v, want %v", cookie.Name, cookie.Secure, secure)
	}
	if cookie.MaxAge != maxAge {
		t.Errorf("cookie %s MaxAge = %d, want %d", cookie.Name, cookie.MaxAge, maxAge)
	}
}

func TestSignInRoundTrip(t *testing.T) {
	for _, appEnv := range []string{"development", "production"} {
		t.Run(appEnv, func(t *testing.T) {
			s := newAuthServer(t, appEnv)

			start := s.do(http.MethodGet, "/api/v1/auth/google", nil, nil)
			if start.Code != http.StatusFound {
				t.Fatalf("start status = %d, want %d", start.Code, http.StatusFound)
			}
			if location := start.Header().Get("Location"); !strings.HasPrefix(location, authtest.AuthURL+"?") {
				t.Fatalf("start redirected to %q, want Google's authorization endpoint", location)
			}
			loginCookie := responseCookie(start, s.cookieName("login"))
			if loginCookie == nil {
				t.Fatal("start did not set the login cookie")
			}
			assertCookieAttributes(t, loginCookie, s.secure, 300)

			state, code := s.google.Authorize(start.Header().Get("Location"), s.account())
			callback := s.callback(loginCookie, url.Values{"state": {state}, "code": {code}})
			if callback.Code != http.StatusSeeOther {
				t.Fatalf("callback status = %d, want %d; body %s", callback.Code, http.StatusSeeOther, callback.Body)
			}
			if location := callback.Header().Get("Location"); location != testFrontendURL {
				t.Errorf("callback redirected to %q, want %q", location, testFrontendURL)
			}

			cleared := responseCookie(callback, s.cookieName("login"))
			if cleared == nil || cleared.MaxAge >= 0 {
				t.Error("callback did not clear the login cookie")
			}
			session := responseCookie(callback, s.cookieName("session"))
			if session == nil {
				t.Fatal("callback did not set the session cookie")
			}
			assertCookieAttributes(t, session, s.secure, 4*60*60)

			me := s.me(session)
			if me.Code != http.StatusOK {
				t.Fatalf("me status = %d, want %d", me.Code, http.StatusOK)
			}
			var body map[string]any
			if err := json.Unmarshal(me.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode me body: %v", err)
			}
			want := map[string]any{
				"id":    s.user.PublicID,
				"name":  s.user.Name,
				"email": s.user.Email,
				"role":  s.user.Role,
			}
			// Exactly these fields: the internal numeric ID must not leak.
			if len(body) != len(want) {
				t.Errorf("me body = %v, want exactly %v", body, want)
			}
			for key, value := range want {
				if body[key] != value {
					t.Errorf("me %s = %v, want %v", key, body[key], value)
				}
			}
		})
	}
}

func TestStartReportsUnavailableWhenFlowCannotBeSaved(t *testing.T) {
	s := newAuthServer(t, "development")
	s.repo.Fail["SaveFlow"] = errors.New("database offline")

	response := s.do(http.MethodGet, "/api/v1/auth/google", nil, nil)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if code := errorCode(t, response); code != "auth_unavailable" {
		t.Errorf("code = %q, want auth_unavailable", code)
	}
	if responseCookie(response, s.cookieName("login")) != nil {
		t.Error("login cookie was set for a flow that was not saved")
	}
	if s.logs.FilterMessage("Cannot start login").Len() != 1 {
		t.Error("failure was not logged")
	}
}

func TestCallbackRejectsRequestsWithoutLoginCookie(t *testing.T) {
	s := newAuthServer(t, "development")
	_, query := s.start(s.account())

	response := s.callback(nil, query)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if code := errorCode(t, response); code != "invalid_login" {
		t.Errorf("code = %q, want invalid_login", code)
	}
	if s.repo.SessionCount() != 0 {
		t.Error("a session was created without the login cookie")
	}
}

func TestCallbackRejectsGoogleError(t *testing.T) {
	s := newAuthServer(t, "development")
	loginCookie, query := s.start(s.account())
	// Google reports a cancelled consent screen with ?error=access_denied.
	query.Set("error", "access_denied")

	response := s.callback(loginCookie, query)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if code := errorCode(t, response); code != "invalid_login" {
		t.Errorf("code = %q, want invalid_login", code)
	}
	if s.repo.SessionCount() != 0 {
		t.Error("a session was created despite Google's error")
	}
}

func TestCallbackRefusalRevealsNothing(t *testing.T) {
	s := newAuthServer(t, "development")
	account := s.account()
	account.Email = "stranger@bitsathy.ac.in"
	loginCookie, query := s.start(account)

	response := s.callback(loginCookie, query)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if code := errorCode(t, response); code != "invalid_login" {
		t.Errorf("code = %q, want invalid_login", code)
	}
	if strings.Contains(response.Body.String(), "stranger") {
		t.Errorf("response %q reveals the refusal reason", response.Body)
	}
	if responseCookie(response, s.cookieName("session")) != nil {
		t.Error("session cookie was set for a refused login")
	}
	if cleared := responseCookie(response, s.cookieName("login")); cleared == nil || cleared.MaxAge >= 0 {
		t.Error("login cookie was not cleared after a refused login")
	}

	refusals := s.logs.FilterMessage("Login refused").All()
	if len(refusals) != 1 {
		t.Fatalf("logged %d refusals, want 1", len(refusals))
	}
	if reason := refusals[0].ContextMap()["error"]; !strings.Contains(reason.(string), "stranger@bitsathy.ac.in") {
		t.Errorf("logged reason %q does not identify the account", reason)
	}
}

func TestCallbackRejectsReplay(t *testing.T) {
	s := newAuthServer(t, "development")
	loginCookie, query := s.start(s.account())

	if first := s.callback(loginCookie, query); first.Code != http.StatusSeeOther {
		t.Fatalf("first callback status = %d, want %d", first.Code, http.StatusSeeOther)
	}
	replay := s.callback(loginCookie, query)

	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed callback status = %d, want %d", replay.Code, http.StatusUnauthorized)
	}
	if s.repo.SessionCount() != 1 {
		t.Errorf("session count = %d, want 1", s.repo.SessionCount())
	}
}

func TestCallbackReportsDatabaseFailureAsUnavailable(t *testing.T) {
	s := newAuthServer(t, "development")
	loginCookie, query := s.start(s.account())
	s.repo.Fail["CreateSession"] = errors.New("database offline")

	response := s.callback(loginCookie, query)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if code := errorCode(t, response); code != "auth_unavailable" {
		t.Errorf("code = %q, want auth_unavailable", code)
	}
	if s.logs.FilterMessage("Login failed unexpectedly").Len() != 1 {
		t.Error("failure was not logged as unexpected")
	}
}

func TestRequireSession(t *testing.T) {
	tests := []struct {
		name       string
		arrange    func(s *authServer, session **http.Cookie)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "no session cookie",
			arrange:    func(_ *authServer, session **http.Cookie) { *session = nil },
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name: "malformed token",
			arrange: func(s *authServer, session **http.Cookie) {
				*session = &http.Cookie{Name: s.cookieName("session"), Value: "garbage"}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthenticated",
		},
		{
			name: "development cookie name in production",
			arrange: func(_ *authServer, session **http.Cookie) {
				(*session).Name = "cyberspace_session"
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "expired session",
			arrange:    func(s *authServer, _ **http.Cookie) { s.repo.ExpireSessions() },
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthenticated",
		},
		{
			name:       "suspended user",
			arrange:    func(s *authServer, _ **http.Cookie) { s.repo.SetStatus(s.user, "suspended") },
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthenticated",
		},
		{
			name: "database failure",
			arrange: func(s *authServer, _ **http.Cookie) {
				s.repo.Fail["ResolveSession"] = errors.New("database offline")
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "auth_unavailable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newAuthServer(t, "production")
			session := s.signIn()
			test.arrange(s, &session)

			response := s.me(session)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if code := errorCode(t, response); code != test.wantCode {
				t.Errorf("code = %q, want %q", code, test.wantCode)
			}
		})
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	s := newAuthServer(t, "development")
	session := s.signIn()

	response := s.logout(session, testFrontendURL)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body %s", response.Code, http.StatusNoContent, response.Body)
	}
	if cleared := responseCookie(response, s.cookieName("session")); cleared == nil || cleared.MaxAge >= 0 {
		t.Error("logout did not clear the session cookie")
	}
	// A copy of the token kept elsewhere must stop working too.
	if me := s.me(session); me.Code != http.StatusUnauthorized {
		t.Errorf("me after logout status = %d, want %d", me.Code, http.StatusUnauthorized)
	}
}

func TestLogoutRejectsForeignOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
	}{
		{name: "cross-site origin", origin: "https://evil.example"},
		{name: "missing origin"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newAuthServer(t, "development")
			session := s.signIn()

			response := s.logout(session, test.origin)

			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
			}
			if me := s.me(session); me.Code != http.StatusOK {
				t.Errorf("session was revoked by a rejected logout; me status = %d", me.Code)
			}
		})
	}
}

func TestLogoutRequiresSession(t *testing.T) {
	s := newAuthServer(t, "development")

	response := s.logout(nil, testFrontendURL)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestLogoutReportsDatabaseFailure(t *testing.T) {
	s := newAuthServer(t, "development")
	session := s.signIn()
	s.repo.Fail["RevokeSession"] = errors.New("database offline")

	response := s.logout(session, testFrontendURL)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if responseCookie(response, s.cookieName("session")) != nil {
		t.Error("session cookie was cleared although the session was not revoked")
	}
}
