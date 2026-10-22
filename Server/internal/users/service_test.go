package users

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/assessments"
)

// fakeRepo records what reaches storage.
type fakeRepo struct {
	imported []StudentRow
	admin    AdminInput
	err      error
}

func (r *fakeRepo) ImportStudents(_ context.Context, rows []StudentRow) (ImportResult, error) {
	r.imported = rows
	return ImportResult{Created: len(rows), Skipped: []RowIssue{}}, r.err
}

func (r *fakeRepo) CreateAdmin(_ context.Context, in AdminInput) (Admin, error) {
	r.admin = in
	return Admin{ID: 7, Name: in.Name, Email: in.Email}, r.err
}

func (r *fakeRepo) Admins(context.Context) ([]Admin, error) { return nil, r.err }

type recordingAudit struct{ entries []assessments.AuditEntry }

func (a *recordingAudit) Audit(_ context.Context, e assessments.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func newTestService(t *testing.T) (*Service, *fakeRepo, *recordingAudit) {
	t.Helper()
	repo, audit := &fakeRepo{}, &recordingAudit{}
	service, err := NewService(repo, audit, zap.NewNop(), Config{AllowedDomain: "Bitsathy.ac.in"})
	if err != nil {
		t.Fatal(err)
	}
	return service, repo, audit
}

// workbook writes sheets of rows (the first row of each is its header).
func workbook(t *testing.T, sheets map[string][][]any, order ...string) *bytes.Buffer {
	t.Helper()
	book := excelize.NewFile()
	defer book.Close()
	for i, name := range order {
		if i == 0 {
			if err := book.SetSheetName("Sheet1", name); err != nil {
				t.Fatal(err)
			}
		} else if _, err := book.NewSheet(name); err != nil {
			t.Fatal(err)
		}
		for r, row := range sheets[name] {
			cell, _ := excelize.CoordinatesToCellName(1, r+1)
			if err := book.SetSheetRow(name, cell, &row); err != nil {
				t.Fatal(err)
			}
		}
	}
	buf, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

var actor = assessments.Actor{UserID: 1}

func TestImportReadsEverySheetByHeaderName(t *testing.T) {
	service, repo, audit := newTestService(t)
	book := workbook(t, map[string][][]any{
		"IV Name List": {
			{"Reg No", "Student Name", "Student Official Email ID", "Department"},
			{"7376222AG102", "AATHITHYA P", "aathithya.ag22@bitsathy.ac.in", "AGRICULTURAL ENGINEERING"},
		},
		// Different headers, different column order, an extra year column,
		// a blank row, and untidy values.
		"III Name List": {
			{"Year", "Dept", "Email ID", "Student Name", "Reg. No."},
			{"III", "AGRI", " BalaManigandanSV.AG24@BITSATHY.AC.IN ", "BALAMANIGANDAN  S V", " 7376242ag101 "},
			{},
			{"III", "MTRS", "tharuns.mz24@bitsathy.ac.in", "THARUN S", "7376251MZ506"},
		},
		"I Name List": {
			{"Roll No", "Name", "Email", "Department"},
			{"2025UAG1024", "VISHNU G", "vishnug.ag25@bitsathy.ac.in", "Agricultural Engineering"},
		},
	}, "IV Name List", "III Name List", "I Name List")

	result, err := service.ImportStudents(context.Background(), actor, book)
	if err != nil {
		t.Fatalf("ImportStudents() error = %v", err)
	}
	want := []StudentRow{
		{"IV Name List", 2, "7376222AG102", "AATHITHYA P", "aathithya.ag22@bitsathy.ac.in", "AGRICULTURAL ENGINEERING"},
		{"III Name List", 2, "7376242AG101", "BALAMANIGANDAN S V", "balamanigandansv.ag24@bitsathy.ac.in", "AGRI"},
		{"III Name List", 4, "7376251MZ506", "THARUN S", "tharuns.mz24@bitsathy.ac.in", "MTRS"},
		{"I Name List", 2, "2025UAG1024", "VISHNU G", "vishnug.ag25@bitsathy.ac.in", "Agricultural Engineering"},
	}
	if len(repo.imported) != len(want) {
		t.Fatalf("imported %d rows %+v; want %d", len(repo.imported), repo.imported, len(want))
	}
	for i := range want {
		if repo.imported[i] != want[i] {
			t.Errorf("row %d = %+v; want %+v", i, repo.imported[i], want[i])
		}
	}
	if result.Created != 4 || len(result.Skipped) != 0 {
		t.Fatalf("result = %+v", result)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "students.import" || audit.entries[0].Outcome != "success" {
		t.Fatalf("audit = %+v", audit.entries)
	}
}

func TestImportSkipsInvalidAndConflictingRows(t *testing.T) {
	service, repo, _ := newTestService(t)
	book := workbook(t, map[string][][]any{
		"A": {
			{"Reg No", "Student Name", "Email", "Department"},
			{"R1", "ONE", "one@bitsathy.ac.in", "IT"},
			{"", "NO REG", "noreg@bitsathy.ac.in", "IT"},
			{"R3", "", "noname@bitsathy.ac.in", "IT"},
			{"R4", "GMAIL", "someone@gmail.com", "IT"},
			{"R5", "BAD", "not an email", "IT"},
			{"R6", "NO DEPT", "nodept@bitsathy.ac.in", ""},
			{"R 7", "SPACE", "space@bitsathy.ac.in", "IT"},
			// Same email, different register number: neither is imported.
			{"R8", "RUPESH J", "rupesh@bitsathy.ac.in", "IT"},
			{"R9", "RUPESH J", "rupesh@bitsathy.ac.in", "IT"},
			// Same register number, different email: neither is imported.
			{"R10", "TEN", "ten@bitsathy.ac.in", "IT"},
			{"R10", "TEN B", "tenb@bitsathy.ac.in", "IT"},
			// Chains: a third row matching a skipped row is skipped too.
			{"R20", "X", "x@bitsathy.ac.in", "IT"},
			{"R20", "Y", "y@bitsathy.ac.in", "IT"},
			{"R21", "Y", "y@bitsathy.ac.in", "IT"},
			{"R30", "P", "p@bitsathy.ac.in", "IT"},
			{"R31", "Q", "q@bitsathy.ac.in", "IT"},
			{"R31", "P", "p@bitsathy.ac.in", "IT"},
			{"R40", "JOSE", "josé@bitsathy.ac.in", "IT"},
		},
		// The same student on two sheets is imported once.
		"B": {
			{"Reg No", "Student Name", "Email", "Department"},
			{"R1", "ONE", "ONE@bitsathy.ac.in", "IT"},
		},
		"Notes": {
			{"nothing here"},
		},
	}, "A", "B", "Notes")

	result, err := service.ImportStudents(context.Background(), actor, book)
	if err != nil {
		t.Fatalf("ImportStudents() error = %v", err)
	}
	if len(repo.imported) != 1 || repo.imported[0].RegisterNumber != "R1" {
		t.Fatalf("imported = %+v; want only R1", repo.imported)
	}

	reasons := map[string]string{}
	for _, issue := range result.Skipped {
		reasons[issue.Sheet+":"+strconv.Itoa(issue.Row)] = issue.Reason
	}
	wantPrefix := map[string]string{
		"A:3":     "Register number is empty",
		"A:4":     "Student name is empty",
		"A:5":     "Email is not on bitsathy.ac.in",
		"A:6":     "Email is not a valid address",
		"A:7":     "Department is empty",
		"A:8":     "Register number may contain only",
		"A:9":     "Conflicts with A row 10",
		"A:10":    "Conflicts with A row 9",
		"A:11":    "Conflicts with A row 12",
		"A:12":    "Conflicts with A row 11",
		"A:13":    "Conflicts with A row 14",
		"A:14":    "Conflicts with A row 13",
		"A:15":    "Conflicts with A row 13",
		"A:16":    "Conflicts with A row 17",
		"A:17":    "Conflicts with A row 16",
		"A:18":    "Conflicts with A row 16",
		"A:19":    "Email may contain only ASCII",
		"Notes:0": "No header row",
	}
	if len(reasons) != len(wantPrefix) {
		t.Errorf("skipped = %+v", result.Skipped)
	}
	for key, prefix := range wantPrefix {
		if !strings.HasPrefix(reasons[key], prefix) {
			t.Errorf("reason for %s = %q; want prefix %q", key, reasons[key], prefix)
		}
	}
}

func TestImportRefusesUnreadableWorkbooks(t *testing.T) {
	service, repo, audit := newTestService(t)
	tests := map[string]*bytes.Buffer{
		"not a workbook": bytes.NewBufferString("Reg No,Name,Email,Department\n"),
		"no header": workbook(t, map[string][][]any{
			"Sheet": {{"7376222AG102", "AATHITHYA P", "a@bitsathy.ac.in", "AG"}},
		}, "Sheet"),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := service.ImportStudents(context.Background(), actor, body)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("ImportStudents() error = %v; want ErrInvalidInput", err)
			}
		})
	}
	if repo.imported != nil {
		t.Fatalf("storage reached with %+v", repo.imported)
	}
	if len(audit.entries) != 2 || audit.entries[0].Outcome != "failure" {
		t.Fatalf("audit = %+v", audit.entries)
	}
}

func TestCreateAdminValidates(t *testing.T) {
	service, repo, audit := newTestService(t)

	admin, err := service.CreateAdmin(context.Background(), actor, AdminInput{Name: "  Dr  Staff ", Email: " Staff@BitSathy.ac.in"})
	if err != nil {
		t.Fatalf("CreateAdmin() error = %v", err)
	}
	if repo.admin != (AdminInput{Name: "Dr Staff", Email: "staff@bitsathy.ac.in"}) || admin.ID != 7 {
		t.Fatalf("stored %+v, returned %+v", repo.admin, admin)
	}
	if audit.entries[0].Action != "admin.create" || audit.entries[0].EntityID != "7" {
		t.Fatalf("audit = %+v", audit.entries[0])
	}

	for _, in := range []AdminInput{
		{Name: "", Email: "a@bitsathy.ac.in"},
		{Name: "A", Email: "a@gmail.com"},
		{Name: "A", Email: "a"},
		{Name: strings.Repeat("x", maxNameLength+1), Email: "a@bitsathy.ac.in"},
	} {
		if _, err := service.CreateAdmin(context.Background(), actor, in); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("CreateAdmin(%+v) error = %v; want ErrInvalidInput", in, err)
		}
	}
}

func TestNewPublicIDIsAULID(t *testing.T) {
	at := time.UnixMilli(1_700_000_000_000)
	a, b := NewPublicID(at), NewPublicID(at)
	if len(a) != 26 || a == b || a[:10] != b[:10] {
		t.Fatalf("ids %q, %q: want 26 characters sharing the time prefix", a, b)
	}
	if later := NewPublicID(at.Add(time.Millisecond)); later[:10] <= a[:10] {
		t.Fatalf("a later id %q does not sort after %q", later, a)
	}
	for _, c := range a {
		if !strings.ContainsRune(crockford, c) {
			t.Fatalf("id %q has %q outside the alphabet", a, c)
		}
	}
	// The first character carries the top three bits only.
	if a[0] > '7' {
		t.Fatalf("id %q overflows 128 bits", a)
	}
}
