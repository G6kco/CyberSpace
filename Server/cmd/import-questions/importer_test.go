package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/G6kco/CyberSpace/internal/database/dbtest"
	"github.com/G6kco/CyberSpace/internal/evaluation"
)

var testKey = bytes.Repeat([]byte("k"), evaluation.MinKeyLength)

// openImportDatabase returns the shared test database, emptied, with the
// admin the import is attributed to.
func openImportDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db := dbtest.Open(t)
	if _, err := db.Exec(
		`INSERT INTO users (public_id, email, display_name, role, status)
        VALUES ('01TESTADMIN000000000000001', 'admin@example.edu', 'Admin', 'admin', 'active')`,
	); err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	return db
}

func testBank(t *testing.T) []question {
	t.Helper()
	bank, err := buildBank(readTestDump(t, testDump))
	if err != nil {
		t.Fatalf("buildBank() error = %v", err)
	}
	return bank
}

func TestImportBankWritesPublishedAssessment(t *testing.T) {
	db := openImportDatabase(t)
	ctx := context.Background()

	result, err := importBank(ctx, db, testKey, "admin@example.edu", testBank(t))
	if err != nil {
		t.Fatalf("importBank() error = %v", err)
	}
	if result != (summary{questions: 3, flags: 5, images: 1}) {
		t.Fatalf("summary = %+v; want 3 questions, 5 flags, 1 image", result)
	}

	var status string
	var points, pass float64
	if err := db.QueryRow(
		`SELECT a.status, v.total_points, v.pass_score
        FROM assessments a JOIN assessment_versions v ON v.id = a.current_version_id
        WHERE a.slug = ?`, assessmentSlug,
	).Scan(&status, &points, &pass); err != nil {
		t.Fatalf("read assessment: %v", err)
	}
	if status != "published" || points != 100 || pass != 80 {
		t.Fatalf("assessment = %s, %v points, pass %v; want published, 100, 80", status, points, pass)
	}

	var image string
	if err := db.QueryRow(
		`SELECT v.yaml_spec FROM assessment_questions q
        JOIN lab_template_versions v ON v.id = q.lab_template_version_id
        WHERE q.code = 'nmap-01'`,
	).Scan(&image); err != nil {
		t.Fatalf("read nmap image: %v", err)
	}
	if image != "image: scenario1\n" {
		t.Fatalf("nmap lab spec = %q", image)
	}

	// The stored digest matches the dump's answer however the student
	// cases it, and the plaintext is stored nowhere.
	var digest []byte
	if err := db.QueryRow(
		`SELECT av.expected_hmac FROM answer_validators av
        JOIN assessment_questions q ON q.id = av.question_id
        WHERE q.code = 'wireshark-01-flag'`,
	).Scan(&digest); err != nil {
		t.Fatalf("read validator: %v", err)
	}
	if !evaluation.Matches(testKey, "d935E3", digest) {
		t.Fatal("stored digest does not match the main flag")
	}
	var plaintext int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM assessment_questions WHERE prompt LIKE '%D935e3%'`,
	).Scan(&plaintext); err != nil || plaintext != 0 {
		t.Fatalf("plaintext flag found in %d prompts (err %v)", plaintext, err)
	}
}

func TestImportBankRunsOnlyOnce(t *testing.T) {
	db := openImportDatabase(t)
	ctx := context.Background()
	if _, err := importBank(ctx, db, testKey, "admin@example.edu", testBank(t)); err != nil {
		t.Fatalf("first importBank() error = %v", err)
	}

	_, err := importBank(ctx, db, testKey, "admin@example.edu", testBank(t))
	if !errors.Is(err, errAlreadyImported) {
		t.Fatalf("second importBank() error = %v; want %v", err, errAlreadyImported)
	}
	var questions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM assessment_questions`).Scan(&questions); err != nil {
		t.Fatalf("count questions: %v", err)
	}
	if questions != 8 { // 3 groups + 5 flags
		t.Fatalf("questions = %d after a repeated import; want 8", questions)
	}
}

func TestImportBankRequiresActiveAdmin(t *testing.T) {
	db := openImportDatabase(t)

	_, err := importBank(context.Background(), db, testKey, "nobody@example.edu", testBank(t))
	if err == nil || !strings.Contains(err.Error(), "no active admin") {
		t.Fatalf("importBank() error = %v; want no active admin", err)
	}
	var levels int
	if err := db.QueryRow(`SELECT COUNT(*) FROM levels`).Scan(&levels); err != nil || levels != 0 {
		t.Fatalf("levels = %d (err %v); want nothing written", levels, err)
	}
}

func TestQuestionPoolConstraints(t *testing.T) {
	db := openImportDatabase(t)
	if _, err := importBank(context.Background(), db, testKey, "admin@example.edu", testBank(t)); err != nil {
		t.Fatalf("importBank() error = %v", err)
	}

	for name, query := range map[string]string{
		"pcap path traversal": `UPDATE assessment_questions SET pcap_file = '../secret.pcap' WHERE code = 'wireshark-01'`,
		"pcap with directory": `UPDATE assessment_questions SET pcap_file = 'PCAP/question1.pcap' WHERE code = 'wireshark-01'`,
		"nmap without image":  `UPDATE assessment_questions SET lab_template_version_id = NULL WHERE code = 'nmap-01'`,
		"pool on a child":     `UPDATE assessment_questions SET pool = 'nmap', difficulty = 'easy' WHERE code = 'wireshark-01-flag'`,
		"unknown difficulty":  `UPDATE assessment_questions SET difficulty = 'medium' WHERE code = 'wireshark-01'`,
	} {
		if _, err := db.Exec(query); err == nil {
			t.Errorf("%s: update succeeded; want a CHECK violation", name)
		}
	}
}
