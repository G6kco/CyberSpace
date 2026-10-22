package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/users"
)

type stubUserRepo struct {
	imported []users.StudentRow
}

func (r *stubUserRepo) ImportStudents(_ context.Context, rows []users.StudentRow) (users.ImportResult, error) {
	r.imported = rows
	return users.ImportResult{Created: len(rows), Skipped: []users.RowIssue{}}, nil
}

func (*stubUserRepo) CreateAdmin(_ context.Context, in users.AdminInput) (users.Admin, error) {
	if in.Email == "taken@bitsathy.ac.in" {
		return users.Admin{}, fmt.Errorf("%w: an account with this email already exists", users.ErrConflict)
	}
	return users.Admin{ID: 3, Name: in.Name, Email: in.Email, Status: "active"}, nil
}

func (*stubUserRepo) Admins(context.Context) ([]users.Admin, error) {
	return []users.Admin{{ID: 1, Name: "Staff", Email: "staff@bitsathy.ac.in", Status: "active"}}, nil
}

func newUserServer(t *testing.T, maxUploadBytes int64) (*authServer, *stubUserRepo) {
	t.Helper()
	server := newAuthServer(t, "development")
	repo := &stubUserRepo{}
	service, err := users.NewService(repo, noAudit{}, zap.NewNop(), users.Config{
		AllowedDomain: testAllowedDomain, MaxUploadBytes: maxUploadBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	server.application.Users = service
	server.router = NewRouter(server.application)
	return server, repo
}

func studentList(t *testing.T) []byte {
	t.Helper()
	book := excelize.NewFile()
	defer book.Close()
	for i, row := range [][]any{
		{"Reg No", "Student Name", "Student Official Email ID", "Department"},
		{"7376222AG102", "AATHITHYA P", "aathithya.ag22@bitsathy.ac.in", "AGRICULTURAL ENGINEERING"},
		{"7376222AG103", "AKHILESH K", "akhilesh.ag22@bitsathy.ac.in", "AGRICULTURAL ENGINEERING"},
	} {
		if err := book.SetSheetRow("Sheet1", fmt.Sprintf("A%d", i+1), &row); err != nil {
			t.Fatal(err)
		}
	}
	buf, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAccountRoutesAreForAdministrators(t *testing.T) {
	s, _ := newUserServer(t, 0)
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
		{"no session", http.MethodGet, "/api/v1/admin/admins", nil, "", "", http.StatusUnauthorized},
		{"student lists", http.MethodGet, "/api/v1/admin/admins", student, "", "", http.StatusForbidden},
		{"student adds", http.MethodPost, "/api/v1/admin/admins", student, testFrontendURL, `{}`, http.StatusForbidden},
		{"student imports", http.MethodPost, "/api/v1/admin/students/import", student, testFrontendURL, "", http.StatusForbidden},
		{"admin lists", http.MethodGet, "/api/v1/admin/admins", admin, "", "", http.StatusOK},
		{"add without origin", http.MethodPost, "/api/v1/admin/admins", admin, "",
			`{"name":"New","email":"new@bitsathy.ac.in"}`, http.StatusForbidden},
		{"add", http.MethodPost, "/api/v1/admin/admins", admin, testFrontendURL,
			`{"name":"New","email":"new@bitsathy.ac.in"}`, http.StatusCreated},
		{"add off-domain", http.MethodPost, "/api/v1/admin/admins", admin, testFrontendURL,
			`{"name":"New","email":"new@gmail.com"}`, http.StatusBadRequest},
		{"add existing", http.MethodPost, "/api/v1/admin/admins", admin, testFrontendURL,
			`{"name":"New","email":"taken@bitsathy.ac.in"}`, http.StatusConflict},
		{"add bad body", http.MethodPost, "/api/v1/admin/admins", admin, testFrontendURL, `[`, http.StatusBadRequest},
		{"import without a file", http.MethodPost, "/api/v1/admin/students/import", admin, testFrontendURL, "", http.StatusBadRequest},
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

func TestImportStudentsUpload(t *testing.T) {
	list := studentList(t)
	s, repo := newUserServer(t, int64(len(list)))
	admin := signInAs(t, s, "staff@bitsathy.ac.in", "admin")

	response := s.upload("/api/v1/admin/students/import", admin, "students.xlsx", list)
	if response.Code != http.StatusOK {
		t.Fatalf("import = %d %s; want 200", response.Code, response.Body)
	}
	var result users.ImportResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Created != 2 || result.Skipped == nil {
		t.Fatalf("result = %s, %v", response.Body, err)
	}
	if len(repo.imported) != 2 || repo.imported[1].Email != "akhilesh.ag22@bitsathy.ac.in" {
		t.Fatalf("imported = %+v", repo.imported)
	}

	response = s.upload("/api/v1/admin/students/import", admin, "students.csv", []byte("Reg No,Name\n"))
	if response.Code != http.StatusBadRequest || errorCode(t, response) != "invalid_input" {
		t.Fatalf("non-workbook upload = %d %s; want 400 invalid_input", response.Code, response.Body)
	}

	response = s.upload("/api/v1/admin/students/import", admin, "big.xlsx", append(list, make([]byte, 2<<20)...))
	if response.Code != http.StatusRequestEntityTooLarge || errorCode(t, response) != "file_too_large" {
		t.Fatalf("oversized upload = %d %s; want 413 file_too_large", response.Code, response.Body)
	}
}
