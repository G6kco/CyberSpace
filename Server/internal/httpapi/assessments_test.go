package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/assessments"
	"github.com/G6kco/CyberSpace/internal/auth/authtest"
)

// stubAssessmentRepo answers the reads the access-control tests reach. Any
// other method panics through the nil embedded interface, which would mean a
// request got past a check it should have failed.
type stubAssessmentRepo struct {
	assessments.Repository
}

func (stubAssessmentRepo) Catalog(context.Context) ([]assessments.Assessment, error) {
	return []assessments.Assessment{{LevelID: 1, LevelName: "Level 1", VersionID: 1, DurationSeconds: 3600, PassScore: 80, TotalPoints: 100}}, nil
}

func (stubAssessmentRepo) StudentState(context.Context, uint64, time.Time) (assessments.StudentState, error) {
	return assessments.StudentState{
		PassedLevels: map[uint64]bool{}, LastFailure: map[uint64]time.Time{}, AttendanceVersions: map[uint64]bool{},
	}, nil
}

func (stubAssessmentRepo) AttendanceUsers(context.Context, time.Time) (map[uint64]uint64, error) {
	return map[uint64]uint64{}, nil
}

func (stubAssessmentRepo) Students(context.Context, []uint64) ([]assessments.Student, error) {
	return []assessments.Student{}, nil
}

type noLabs struct{}

func (noLabs) RequestProvision(context.Context, uint64, uint64) (uint64, error) { return 0, nil }
func (noLabs) RequestStop(context.Context, uint64) error                        { return nil }

func newAssessmentServer(t *testing.T) *authServer {
	t.Helper()
	server := newAuthServer(t, "development")
	service, err := assessments.NewService(stubAssessmentRepo{}, noLabs{}, zap.NewNop(),
		assessments.Config{AnswerKey: bytes.Repeat([]byte("k"), 32)})
	if err != nil {
		t.Fatal(err)
	}

	// Rebuild the router with the assessment routes on the same login.
	server.application.Assessments = service
	server.router = NewRouter(server.application)
	return server
}

// signInAs signs in a new account of role.
func signInAs(t *testing.T, s *authServer, email, role string) *http.Cookie {
	t.Helper()
	s.repo.AddUser(email, role, "active")
	response := s.callback(s.start(authtest.Account{
		Subject: "subject-" + email, Email: email, EmailVerified: true, HostedDomain: testAllowedDomain,
	}))
	session := responseCookie(response, s.cookieName("session"))
	if session == nil {
		t.Fatalf("sign-in as %s failed: %d %s", email, response.Code, response.Body)
	}
	return session
}

func (s *authServer) request(method, target string, session *http.Cookie, origin string, body string) *httptest.ResponseRecorder {
	s.t.Helper()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request = request.WithContext(s.google.Context(request.Context()))
	request.Header.Set("Content-Type", "application/json")
	if session != nil {
		request.AddCookie(session)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

func TestAssessmentRoutesRequireSession(t *testing.T) {
	s := newAssessmentServer(t)
	for _, target := range []string{"/api/v1/student/assessment", "/api/v1/admin/lobby", "/api/v1/admin/attempts"} {
		if response := s.request(http.MethodGet, target, nil, "", ""); response.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without session = %d; want 401", target, response.Code)
		}
	}
}

func TestAssessmentRoutesEnforceRoles(t *testing.T) {
	s := newAssessmentServer(t)
	student := signInAs(t, s, "pupil@bitsathy.ac.in", "student")
	admin := signInAs(t, s, "staff@bitsathy.ac.in", "admin")

	tests := []struct {
		name    string
		method  string
		target  string
		session *http.Cookie
		body    string
		want    int
	}{
		{"student overview", http.MethodGet, "/api/v1/student/assessment", student, "", http.StatusOK},
		{"student heartbeat", http.MethodPost, "/api/v1/student/lobby/heartbeat", student, "", http.StatusOK},
		{"student leaves lobby", http.MethodDelete, "/api/v1/student/lobby", student, "", http.StatusNoContent},
		{"student on admin lobby", http.MethodGet, "/api/v1/admin/lobby", student, "", http.StatusForbidden},
		{"student starts a test", http.MethodPost, "/api/v1/admin/lobby/start", student, `{"studentIds":["x"]}`, http.StatusForbidden},
		{"student revokes", http.MethodPost, "/api/v1/admin/attempts/1/revoke", student, `{"reason":"x"}`, http.StatusForbidden},
		{"student reads question bank", http.MethodGet, "/api/v1/admin/question-bank", student, "", http.StatusForbidden},
		{"admin lobby", http.MethodGet, "/api/v1/admin/lobby", admin, "", http.StatusOK},
		{"admin on student overview", http.MethodGet, "/api/v1/student/assessment", admin, "", http.StatusForbidden},
		{"admin submits a flag", http.MethodPost, "/api/v1/student/attempts/1/answers", admin, `{"questionId":1,"answer":"x"}`, http.StatusForbidden},
		{"bad attempt id", http.MethodGet, "/api/v1/student/attempts/abc", student, "", http.StatusNotFound},
		{"empty start body", http.MethodPost, "/api/v1/admin/lobby/start", admin, `{"studentIds":[]}`, http.StatusBadRequest},
		{"bad scope", http.MethodGet, "/api/v1/admin/attempts?scope=all", admin, "", http.StatusBadRequest},
		{"bad answer body", http.MethodPost, "/api/v1/student/attempts/1/answers", student, `{"answer":"x"}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := s.request(tt.method, tt.target, tt.session, testFrontendURL, tt.body)
			if response.Code != tt.want {
				t.Fatalf("%s %s = %d; want %d; body %s", tt.method, tt.target, response.Code, tt.want, response.Body)
			}
		})
	}
}

func TestStateChangingAssessmentRoutesRequireOrigin(t *testing.T) {
	s := newAssessmentServer(t)
	student := signInAs(t, s, "pupil@bitsathy.ac.in", "student")

	// Without an Origin, requireOrigin refuses the request.
	response := s.request(http.MethodPost, "/api/v1/student/lobby/heartbeat", student, "", "")
	if response.Code != http.StatusForbidden || errorCode(t, response) != "bad_origin" {
		t.Errorf("heartbeat without Origin = %d %s; want 403 bad_origin", response.Code, response.Body)
	}
	// A foreign Origin is already refused by the CORS middleware.
	response = s.request(http.MethodPost, "/api/v1/student/lobby/heartbeat", student, "https://evil.example", "")
	if response.Code != http.StatusForbidden {
		t.Errorf("heartbeat from a foreign origin = %d; want 403", response.Code)
	}
	// Reads need no Origin: browsers omit it on same-origin GETs.
	if response := s.request(http.MethodGet, "/api/v1/student/assessment", student, "", ""); response.Code != http.StatusOK {
		t.Fatalf("overview without Origin = %d; want 200", response.Code)
	}
}
