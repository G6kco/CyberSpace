package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/G6kco/CyberSpace/internal/auth"
	"github.com/G6kco/CyberSpace/internal/auth/authtest"
)

const allowedDomain = "bitsathy.ac.in"

var errDatabase = errors.New("database offline")

type fixture struct {
	ctx    context.Context
	google *authtest.Google
	repo   *authtest.MemoryRepository
	login  *auth.GoogleLogin
	user   *authtest.UserRecord
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	google := authtest.NewGoogle(t)
	repo := authtest.NewMemoryRepository()
	ctx := google.Context(context.Background())

	login, err := auth.NewGoogleLogin(ctx, authtest.Config(allowedDomain), repo)
	if err != nil {
		t.Fatalf("NewGoogleLogin() error = %v", err)
	}

	return &fixture{
		ctx:    ctx,
		google: google,
		repo:   repo,
		login:  login,
		user:   repo.AddUser("student@bitsathy.ac.in", "student", "active"),
	}
}

func (f *fixture) account() authtest.Account {
	return authtest.Account{
		Subject:       "google-subject-1",
		Email:         f.user.Email,
		EmailVerified: true,
		HostedDomain:  allowedDomain,
	}
}

// callback runs Start and the browser's trip to Google, returning what the
// callback handler would receive.
func (f *fixture) callback(t *testing.T, account authtest.Account) (state, browserSecret, code string) {
	t.Helper()

	authURL, browserSecret, err := f.login.Start(f.ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	state, code = f.google.Authorize(authURL, account)
	return state, browserSecret, code
}

func (f *fixture) signIn(t *testing.T) string {
	t.Helper()

	state, browserSecret, code := f.callback(t, f.account())
	token, err := f.login.Complete(f.ctx, state, browserSecret, code)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	return token
}

func TestNewGoogleLoginFailsWhenGoogleIsUnreachable(t *testing.T) {
	ctx := authtest.Unreachable(context.Background())

	login, err := auth.NewGoogleLogin(ctx, authtest.Config(allowedDomain), authtest.NewMemoryRepository())
	if err == nil {
		t.Fatal("NewGoogleLogin() error = nil, want a discovery error")
	}
	if login != nil {
		t.Error("NewGoogleLogin() returned a login despite failing discovery")
	}
}

func TestSessionTTLIsFourHours(t *testing.T) {
	f := newFixture(t)

	if got := f.login.SessionTTL(); got != 4*time.Hour {
		t.Errorf("SessionTTL() = %v, want 4h", got)
	}
}

func TestStartBuildsGoogleAuthorizationRequest(t *testing.T) {
	f := newFixture(t)

	authURL, browserSecret, err := f.login.Start(f.ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	// Regression: without the discovered endpoint, AuthCodeURL returned a
	// bare "?client_id=..." that the browser resolved against the API.
	if got := parsed.Scheme + "://" + parsed.Host + parsed.Path; got != authtest.AuthURL {
		t.Fatalf("authorization endpoint = %q, want %q", got, authtest.AuthURL)
	}

	query := parsed.Query()
	for _, scope := range []string{"openid", "email", "profile"} {
		if !authtest.HasScope(query.Get("scope"), scope) {
			t.Errorf("scope %q missing from %q", scope, query.Get("scope"))
		}
	}

	flows := f.repo.Flows()
	if len(flows) != 1 {
		t.Fatalf("saved %d flows, want 1", len(flows))
	}
	flow := flows[0]

	// Only hashes of the state and browser secret are stored.
	if flow.StateHash != sha256.Sum256([]byte(query.Get("state"))) {
		t.Error("stored state hash does not match the state sent to Google")
	}
	if flow.BrowserHash != sha256.Sum256([]byte(browserSecret)) {
		t.Error("stored browser hash does not match the returned browser secret")
	}
	if flow.Nonce != query.Get("nonce") {
		t.Errorf("stored nonce = %q, want the nonce sent to Google %q", flow.Nonce, query.Get("nonce"))
	}

	verifierHash := sha256.Sum256([]byte(flow.Verifier))
	if base64.RawURLEncoding.EncodeToString(verifierHash[:]) != query.Get("code_challenge") {
		t.Error("code_challenge is not the S256 hash of the stored PKCE verifier")
	}

	if remaining := time.Until(flow.ExpiresAt); remaining <= 4*time.Minute || remaining > 5*time.Minute {
		t.Errorf("flow expires in %v, want about 5m", remaining)
	}

	for name, value := range map[string]string{
		"state":          query.Get("state"),
		"browser secret": browserSecret,
		"nonce":          flow.Nonce,
	} {
		if len(value) != 43 {
			t.Errorf("%s has length %d, want 43 (32 random bytes)", name, len(value))
		}
	}
	if browserSecret == query.Get("state") {
		t.Error("browser secret equals the state; it must be independent")
	}
}

func TestStartUsesFreshValuesEachTime(t *testing.T) {
	f := newFixture(t)

	first, firstSecret, err := f.login.Start(f.ctx)
	if err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	second, secondSecret, err := f.login.Start(f.ctx)
	if err != nil {
		t.Fatalf("second Start() error = %v", err)
	}

	if first == second || firstSecret == secondSecret {
		t.Error("two Start() calls produced the same URL or browser secret")
	}
}

func TestStartReportsRepositoryFailure(t *testing.T) {
	f := newFixture(t)
	f.repo.Fail["SaveFlow"] = errDatabase

	authURL, browserSecret, err := f.login.Start(f.ctx)
	if !errors.Is(err, errDatabase) {
		t.Fatalf("Start() error = %v, want %v", err, errDatabase)
	}
	if authURL != "" || browserSecret != "" {
		t.Error("Start() returned a URL or secret for a flow that was not saved")
	}
}

func TestCompleteSignsInProvisionedUser(t *testing.T) {
	f := newFixture(t)

	token := f.signIn(t)

	if len(token) != 43 {
		t.Errorf("session token length = %d, want 43", len(token))
	}
	session, ok := f.repo.Session(sha256.Sum256([]byte(token)))
	if !ok {
		t.Fatal("no session stored under the hash of the returned token")
	}
	if session.UserID != f.user.ID {
		t.Errorf("session user = %d, want %d", session.UserID, f.user.ID)
	}
	if remaining := time.Until(session.ExpiresAt); remaining <= 4*time.Hour-time.Minute || remaining > 4*time.Hour {
		t.Errorf("session expires in %v, want about 4h", remaining)
	}
	if f.user.GoogleSubject != "google-subject-1" {
		t.Errorf("bound subject = %q, want %q", f.user.GoogleSubject, "google-subject-1")
	}

	user, err := f.login.CurrentUser(f.ctx, token)
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if user.Email != f.user.Email {
		t.Errorf("CurrentUser() email = %q, want %q", user.Email, f.user.Email)
	}
}

func TestCompleteAcceptsHostedDomainInAnyCase(t *testing.T) {
	f := newFixture(t)
	account := f.account()
	account.HostedDomain = "BITSathy.AC.IN"

	state, browserSecret, code := f.callback(t, account)
	if _, err := f.login.Complete(f.ctx, state, browserSecret, code); err != nil {
		t.Fatalf("Complete() error = %v, want success", err)
	}
}

func TestCompleteSignsInAgainWithBoundSubject(t *testing.T) {
	f := newFixture(t)
	f.signIn(t)

	// A later login is matched by subject, so it survives an email change.
	account := f.account()
	account.Email = "renamed@bitsathy.ac.in"
	state, browserSecret, code := f.callback(t, account)
	if _, err := f.login.Complete(f.ctx, state, browserSecret, code); err != nil {
		t.Fatalf("second Complete() error = %v", err)
	}
	if f.repo.SessionCount() != 2 {
		t.Errorf("session count = %d, want 2", f.repo.SessionCount())
	}
}

type callbackValues struct {
	state, browserSecret, code string
}

func TestCompleteRefusals(t *testing.T) {
	tests := []struct {
		name string
		// before runs before Google issues the code: it shapes the account
		// Google vouches for and the provisioned users.
		before func(f *fixture, account *authtest.Account)
		// tamper edits what reaches the callback.
		tamper func(t *testing.T, f *fixture, values *callbackValues)
		want   error
	}{
		{
			name:   "missing state",
			tamper: func(_ *testing.T, _ *fixture, v *callbackValues) { v.state = "" },
			want:   auth.ErrInvalidFlow,
		},
		{
			name:   "missing browser cookie",
			tamper: func(_ *testing.T, _ *fixture, v *callbackValues) { v.browserSecret = "" },
			want:   auth.ErrInvalidFlow,
		},
		{
			name:   "missing code",
			tamper: func(_ *testing.T, _ *fixture, v *callbackValues) { v.code = "" },
			want:   auth.ErrInvalidFlow,
		},
		{
			name:   "wrong state",
			tamper: func(_ *testing.T, _ *fixture, v *callbackValues) { v.state += "x" },
			want:   auth.ErrInvalidFlow,
		},
		{
			name: "browser cookie from another login",
			tamper: func(t *testing.T, f *fixture, v *callbackValues) {
				_, other, err := f.login.Start(f.ctx)
				if err != nil {
					t.Fatalf("Start() error = %v", err)
				}
				v.browserSecret = other
			},
			want: auth.ErrInvalidFlow,
		},
		{
			name:   "expired login flow",
			tamper: func(_ *testing.T, f *fixture, _ *callbackValues) { f.repo.ExpireFlows() },
			want:   auth.ErrInvalidFlow,
		},
		{
			name:   "code rejected by Google",
			tamper: func(_ *testing.T, _ *fixture, v *callbackValues) { v.code = "forged-code" },
			want:   auth.ErrAccessDenied,
		},
		{
			name:   "token response without id_token",
			before: func(_ *fixture, a *authtest.Account) { a.OmitIDToken = true },
			want:   auth.ErrAccessDenied,
		},
		{
			name:   "id_token with a forged signature",
			before: func(_ *fixture, a *authtest.Account) { a.SignWithUnknownKey = true },
			want:   auth.ErrAccessDenied,
		},
		{
			name: "id_token for another client",
			before: func(_ *fixture, a *authtest.Account) {
				a.Mutate = func(claims map[string]any) { claims["aud"] = "someone-else" }
			},
			want: auth.ErrAccessDenied,
		},
		{
			name: "id_token from another issuer",
			before: func(_ *fixture, a *authtest.Account) {
				a.Mutate = func(claims map[string]any) { claims["iss"] = "https://evil.example" }
			},
			want: auth.ErrAccessDenied,
		},
		{
			name: "expired id_token",
			before: func(_ *fixture, a *authtest.Account) {
				a.Mutate = func(claims map[string]any) { claims["exp"] = time.Now().Add(-time.Hour).Unix() }
			},
			want: auth.ErrAccessDenied,
		},
		{
			name: "id_token nonce from another login",
			before: func(_ *fixture, a *authtest.Account) {
				a.Mutate = func(claims map[string]any) { claims["nonce"] = "replayed-nonce" }
			},
			want: auth.ErrAccessDenied,
		},
		{
			name:   "unverified email",
			before: func(_ *fixture, a *authtest.Account) { a.EmailVerified = false },
			want:   auth.ErrAccessDenied,
		},
		{
			name:   "personal Google account",
			before: func(_ *fixture, a *authtest.Account) { a.HostedDomain = "" },
			want:   auth.ErrAccessDenied,
		},
		{
			name:   "other Workspace domain",
			before: func(_ *fixture, a *authtest.Account) { a.HostedDomain = "other.edu" },
			want:   auth.ErrAccessDenied,
		},
		{
			name:   "unknown user",
			before: func(_ *fixture, a *authtest.Account) { a.Email = "stranger@bitsathy.ac.in" },
			want:   auth.ErrAccessDenied,
		},
		{
			name:   "suspended user",
			before: func(f *fixture, _ *authtest.Account) { f.repo.SetStatus(f.user, "suspended") },
			want:   auth.ErrAccessDenied,
		},
		{
			name:   "archived user",
			before: func(f *fixture, _ *authtest.Account) { f.repo.SetStatus(f.user, "archived") },
			want:   auth.ErrAccessDenied,
		},
		{
			name:   "user bound to another Google account",
			before: func(f *fixture, _ *authtest.Account) { f.user.GoogleSubject = "google-subject-other" },
			want:   auth.ErrAccessDenied,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			account := f.account()
			if test.before != nil {
				test.before(f, &account)
			}

			var values callbackValues
			values.state, values.browserSecret, values.code = f.callback(t, account)
			if test.tamper != nil {
				test.tamper(t, f, &values)
			}

			token, err := f.login.Complete(f.ctx, values.state, values.browserSecret, values.code)
			if !errors.Is(err, test.want) {
				t.Fatalf("Complete() error = %v, want %v", err, test.want)
			}
			if token != "" {
				t.Error("Complete() returned a session token for a refused login")
			}
			if f.repo.SessionCount() != 0 {
				t.Error("a session was created for a refused login")
			}
		})
	}
}

func TestCompleteRejectsReusedCallback(t *testing.T) {
	f := newFixture(t)
	state, browserSecret, code := f.callback(t, f.account())

	if _, err := f.login.Complete(f.ctx, state, browserSecret, code); err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	_, err := f.login.Complete(f.ctx, state, browserSecret, code)
	if !errors.Is(err, auth.ErrInvalidFlow) {
		t.Fatalf("replayed Complete() error = %v, want %v", err, auth.ErrInvalidFlow)
	}
	if f.repo.SessionCount() != 1 {
		t.Errorf("session count = %d, want 1", f.repo.SessionCount())
	}
}

func TestCompleteFlowSurvivesWrongBrowserCookie(t *testing.T) {
	f := newFixture(t)
	state, browserSecret, code := f.callback(t, f.account())

	// A request with the right state but another browser's cookie must not
	// burn the flow belonging to the browser that started it.
	if _, err := f.login.Complete(f.ctx, state, strings.Repeat("A", 43), code); !errors.Is(err, auth.ErrInvalidFlow) {
		t.Fatalf("Complete() with wrong cookie error = %v, want %v", err, auth.ErrInvalidFlow)
	}
	if _, err := f.login.Complete(f.ctx, state, browserSecret, code); err != nil {
		t.Fatalf("Complete() from the original browser error = %v", err)
	}
}

func TestCompleteReportsDatabaseFailureAsFault(t *testing.T) {
	for _, step := range []string{"ConsumeFlow", "FindOrBindUser", "CreateSession"} {
		t.Run(step, func(t *testing.T) {
			f := newFixture(t)
			state, browserSecret, code := f.callback(t, f.account())
			f.repo.Fail[step] = errDatabase

			token, err := f.login.Complete(f.ctx, state, browserSecret, code)
			if !errors.Is(err, errDatabase) {
				t.Fatalf("Complete() error = %v, want %v", err, errDatabase)
			}
			// A fault must not be classified as a refusal, or the handler
			// would answer 401 and hide an outage.
			if errors.Is(err, auth.ErrAccessDenied) || errors.Is(err, auth.ErrInvalidFlow) {
				t.Errorf("database fault %v was classified as a refusal", err)
			}
			if token != "" {
				t.Error("Complete() returned a token despite the failure")
			}
		})
	}
}

func TestCurrentUser(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(f *fixture, token *string)
		want    error
	}{
		{name: "active session"},
		{
			name:    "empty token",
			arrange: func(_ *fixture, token *string) { *token = "" },
			want:    auth.ErrUnauthenticated,
		},
		{
			name:    "truncated token",
			arrange: func(_ *fixture, token *string) { *token = (*token)[:42] },
			want:    auth.ErrUnauthenticated,
		},
		{
			name:    "oversized token",
			arrange: func(_ *fixture, token *string) { *token += "A" },
			want:    auth.ErrUnauthenticated,
		},
		{
			name:    "unknown token",
			arrange: func(_ *fixture, token *string) { *token = strings.Repeat("A", 43) },
			want:    auth.ErrUnauthenticated,
		},
		{
			name:    "expired session",
			arrange: func(f *fixture, _ *string) { f.repo.ExpireSessions() },
			want:    auth.ErrUnauthenticated,
		},
		{
			name: "revoked session",
			// SignOut's own errors are covered by TestSignOut.
			arrange: func(f *fixture, token *string) { _ = f.login.SignOut(f.ctx, *token) },
			want:    auth.ErrUnauthenticated,
		},
		{
			name:    "user suspended after sign-in",
			arrange: func(f *fixture, _ *string) { f.repo.SetStatus(f.user, "suspended") },
			want:    auth.ErrUnauthenticated,
		},
		{
			name:    "database failure",
			arrange: func(f *fixture, _ *string) { f.repo.Fail["ResolveSession"] = errDatabase },
			want:    errDatabase,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			token := f.signIn(t)
			if test.arrange != nil {
				test.arrange(f, &token)
			}

			user, err := f.login.CurrentUser(f.ctx, token)
			if !errors.Is(err, test.want) {
				t.Fatalf("CurrentUser() error = %v, want %v", err, test.want)
			}
			if test.want == nil && user.ID != f.user.ID {
				t.Errorf("CurrentUser() user = %d, want %d", user.ID, f.user.ID)
			}
		})
	}
}

func TestCurrentUserRejectsMalformedTokenWithoutDatabase(t *testing.T) {
	f := newFixture(t)
	// If the length check were skipped, this failure would surface.
	f.repo.Fail["ResolveSession"] = errDatabase

	if _, err := f.login.CurrentUser(f.ctx, "short"); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("CurrentUser() error = %v, want %v", err, auth.ErrUnauthenticated)
	}
}

func TestSignOut(t *testing.T) {
	t.Run("revokes the session", func(t *testing.T) {
		f := newFixture(t)
		token := f.signIn(t)

		if err := f.login.SignOut(f.ctx, token); err != nil {
			t.Fatalf("SignOut() error = %v", err)
		}
		session, _ := f.repo.Session(sha256.Sum256([]byte(token)))
		if !session.Revoked {
			t.Error("session was not revoked")
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		f := newFixture(t)
		token := f.signIn(t)

		for attempt := 1; attempt <= 2; attempt++ {
			if err := f.login.SignOut(f.ctx, token); err != nil {
				t.Fatalf("SignOut() attempt %d error = %v", attempt, err)
			}
		}
	})

	t.Run("ignores an unknown token", func(t *testing.T) {
		f := newFixture(t)
		if err := f.login.SignOut(f.ctx, strings.Repeat("A", 43)); err != nil {
			t.Fatalf("SignOut() error = %v, want nil", err)
		}
	})

	t.Run("treats an already invalid session as signed out", func(t *testing.T) {
		f := newFixture(t)
		f.repo.Fail["RevokeSession"] = auth.ErrUnauthenticated
		if err := f.login.SignOut(f.ctx, strings.Repeat("A", 43)); err != nil {
			t.Fatalf("SignOut() error = %v, want nil", err)
		}
	})

	t.Run("rejects an empty token", func(t *testing.T) {
		f := newFixture(t)
		if err := f.login.SignOut(f.ctx, ""); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Fatalf("SignOut() error = %v, want %v", err, auth.ErrUnauthenticated)
		}
	})

	t.Run("reports a database failure", func(t *testing.T) {
		f := newFixture(t)
		f.repo.Fail["RevokeSession"] = errDatabase
		if err := f.login.SignOut(f.ctx, strings.Repeat("A", 43)); !errors.Is(err, errDatabase) {
			t.Fatalf("SignOut() error = %v, want %v", err, errDatabase)
		}
	})
}
