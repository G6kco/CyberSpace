package httpapi

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/assessments"
	"github.com/G6kco/CyberSpace/internal/levels"
)

// stubLevelRepo answers the calls the route tests reach. Any other method
// panics through the nil embedded interface.
type stubLevelRepo struct {
	levels.Repository
	added []levels.NewFile
}

func (*stubLevelRepo) List(context.Context) ([]levels.Level, error) {
	return []levels.Level{{ID: 2, Name: "Level 2", Built: true, Status: levels.StatusDraft}}, nil
}

func (*stubLevelRepo) Get(_ context.Context, id uint64) (levels.Level, error) {
	if id != 2 {
		return levels.Level{}, levels.ErrNotFound
	}
	return levels.Level{ID: 2, Name: "Level 2", Built: true, Status: levels.StatusDraft}, nil
}

func (r *stubLevelRepo) AddFile(_ context.Context, _, _ uint64, f levels.NewFile) (uint64, error) {
	r.added = append(r.added, f)
	return 9, nil
}

type noAudit struct{}

func (noAudit) Audit(context.Context, assessments.AuditEntry) error { return nil }

func newLevelServer(t *testing.T, maxFileBytes int64) (*authServer, *stubLevelRepo, string) {
	t.Helper()
	server := newAuthServer(t, "development")
	repo := &stubLevelRepo{}
	dir := t.TempDir()
	service, err := levels.NewService(repo, noAudit{}, zap.NewNop(), levels.Config{
		AnswerKey: bytes.Repeat([]byte("k"), 32), FilesDir: dir, MaxFileBytes: maxFileBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	server.application.Levels = service
	server.router = NewRouter(server.application)
	return server, repo, dir
}

func TestLevelRoutesAreForAdministrators(t *testing.T) {
	s, _, _ := newLevelServer(t, 0)
	student := signInAs(t, s, "pupil@bitsathy.ac.in", "student")
	admin := signInAs(t, s, "staff@bitsathy.ac.in", "admin")

	tests := []struct {
		name    string
		method  string
		target  string
		session *http.Cookie
		origin  string
		body    string
		want    int
	}{
		{"no session", http.MethodGet, "/api/v1/admin/levels", nil, "", "", http.StatusUnauthorized},
		{"student lists", http.MethodGet, "/api/v1/admin/levels", student, "", "", http.StatusForbidden},
		{"student creates", http.MethodPost, "/api/v1/admin/levels", student, testFrontendURL, `{}`, http.StatusForbidden},
		{"admin lists", http.MethodGet, "/api/v1/admin/levels", admin, "", "", http.StatusOK},
		{"admin reads", http.MethodGet, "/api/v1/admin/levels/2", admin, "", "", http.StatusOK},
		{"unknown level", http.MethodGet, "/api/v1/admin/levels/3", admin, "", "", http.StatusNotFound},
		{"create without origin", http.MethodPost, "/api/v1/admin/levels", admin, "", `{}`, http.StatusForbidden},
		{"create with bad settings", http.MethodPost, "/api/v1/admin/levels", admin, testFrontendURL,
			`{"name":"Level 2","durationMinutes":0,"questionsPerAttempt":3,"passPercent":60}`, http.StatusBadRequest},
		{"create with bad body", http.MethodPost, "/api/v1/admin/levels", admin, testFrontendURL, `[`, http.StatusBadRequest},
		{"question with bad image", http.MethodPost, "/api/v1/admin/levels/2/questions", admin, testFrontendURL,
			`{"prompt":"p","image":"Web App","answers":[{"prompt":"a","answer":"b","marks":1}]}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := s.request(tt.method, tt.target, tt.session, tt.origin, tt.body)
			if response.Code != tt.want {
				t.Fatalf("%s %s = %d; want %d; body %s", tt.method, tt.target, response.Code, tt.want, response.Body)
			}
		})
	}
}

func (s *authServer) upload(target string, session *http.Cookie, name string, content []byte) *httptest.ResponseRecorder {
	s.t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("note", "ignored")
	part, _ := form.CreateFormFile("file", name)
	_, _ = part.Write(content)
	_ = form.Close()

	request := httptest.NewRequest(http.MethodPost, target, &body)
	request = request.WithContext(s.google.Context(request.Context()))
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Origin", testFrontendURL)
	request.AddCookie(session)
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

func TestUploadStreamsFileAndRefusesOversize(t *testing.T) {
	s, repo, dir := newLevelServer(t, 32)
	admin := signInAs(t, s, "staff@bitsathy.ac.in", "admin")

	response := s.upload("/api/v1/admin/levels/2/questions/5/files", admin, "capture.pcap", []byte("pcap bytes"))
	if response.Code != http.StatusCreated {
		t.Fatalf("upload = %d %s; want 201", response.Code, response.Body)
	}
	if len(repo.added) != 1 || repo.added[0].OriginalName != "capture.pcap" ||
		repo.added[0].QuestionID != 5 || repo.added[0].Size != 10 {
		t.Fatalf("recorded files = %+v", repo.added)
	}
	stored, err := os.ReadFile(dir + "/" + repo.added[0].StoredName)
	if err != nil || string(stored) != "pcap bytes" {
		t.Fatalf("stored file = %q, %v", stored, err)
	}

	response = s.upload("/api/v1/admin/levels/2/questions/5/files", admin, "big.pcap", bytes.Repeat([]byte("x"), 33))
	if response.Code != http.StatusRequestEntityTooLarge || errorCode(t, response) != "file_too_large" {
		t.Fatalf("oversized upload = %d %s; want 413 file_too_large", response.Code, response.Body)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("files on disk after a refused upload = %d; want 1", len(entries))
	}
}
