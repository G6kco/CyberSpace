package database

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/G6kco/CyberSpace/internal/database/dbtest"
	"github.com/G6kco/CyberSpace/internal/users"
)

func student(reg, name, email, dept string) users.StudentRow {
	return users.StudentRow{Sheet: "IV", Row: 2, RegisterNumber: reg, Name: name, Email: email, Department: dept}
}

func TestImportStudentsCreatesUpdatesAndSkips(t *testing.T) {
	db := dbtest.Open(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	insertUserWithRole(t, db, "staff@bitsathy.ac.in", "admin")
	// A suspended student with no profile, and one with a profile.
	suspended := insertUserWithRole(t, db, "old@bitsathy.ac.in", "student")
	exec(t, db, `UPDATE users SET status = 'suspended' WHERE id = ?`, suspended)
	owner := insertUserWithRole(t, db, "owner@bitsathy.ac.in", "student")
	exec(t, db, `INSERT INTO student_profiles (user_id, register_number, department, programme, graduation_year)
        VALUES (?, 'R-OWNED', 'CSE', 'B.E.', 2026)`, owner)

	rows := []users.StudentRow{
		student("R1", "ONE", "one@bitsathy.ac.in", "IT"),
		student("R2", "TWO", "two@bitsathy.ac.in", "IT"),
		student("R-OLD", "OLD NAME", "old@bitsathy.ac.in", "MECH"),
		student("R-STAFF", "STAFF", "staff@bitsathy.ac.in", "IT"),
		student("R-OWNED", "THIEF", "thief@bitsathy.ac.in", "IT"),
		// The owner keeps their register number; their department changes.
		student("R-OWNED", "User owner@bitsathy.ac.in", "owner@bitsathy.ac.in", "ECE"),
	}
	result, err := repo.ImportStudents(ctx, rows)
	if err != nil {
		t.Fatalf("ImportStudents() error = %v", err)
	}
	if result.Created != 2 || result.Updated != 2 || result.Unchanged != 0 || len(result.Skipped) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if result.Skipped[0].Reason != "This email belongs to an administrator" ||
		result.Skipped[1].Reason != "Register number already belongs to owner@bitsathy.ac.in" {
		t.Fatalf("skipped = %+v", result.Skipped)
	}

	var name, role, status, reg, dept, publicID string
	err = db.QueryRow(`SELECT u.display_name, u.role, u.status, u.public_id, sp.register_number, sp.department
        FROM users u JOIN student_profiles sp ON sp.user_id = u.id WHERE u.email = 'two@bitsathy.ac.in'`).
		Scan(&name, &role, &status, &publicID, &reg, &dept)
	if err != nil || name != "TWO" || role != "student" || status != "active" || reg != "R2" || dept != "IT" || len(publicID) != 26 {
		t.Fatalf("created student = %q %q %q %q %q %q, %v", name, role, status, publicID, reg, dept, err)
	}
	err = db.QueryRow(`SELECT u.display_name, u.status, sp.register_number, sp.department
        FROM users u JOIN student_profiles sp ON sp.user_id = u.id WHERE u.id = ?`, suspended).
		Scan(&name, &status, &reg, &dept)
	if err != nil || name != "OLD NAME" || status != "suspended" || reg != "R-OLD" || dept != "MECH" {
		t.Fatalf("updated student = %q %q %q %q, %v; want the profile added and the suspension kept", name, status, reg, dept, err)
	}
	var programme string
	err = db.QueryRow(`SELECT department, programme FROM student_profiles WHERE user_id = ?`, owner).Scan(&dept, &programme)
	if err != nil || dept != "ECE" || programme != "B.E." {
		t.Fatalf("owner profile = %q %q, %v", dept, programme, err)
	}

	// Uploading the same list again changes nothing.
	again, err := repo.ImportStudents(ctx, rows)
	if err != nil || again.Created != 0 || again.Updated != 0 || again.Unchanged != 4 || len(again.Skipped) != 2 {
		t.Fatalf("second import = %+v, %v", again, err)
	}
}

func TestImportStudentsInChunks(t *testing.T) {
	db := dbtest.Open(t)
	repo := NewUserRepository(db)

	rows := make([]users.StudentRow, 2*importChunk+7)
	for i := range rows {
		rows[i] = student(fmt.Sprintf("R%04d", i), fmt.Sprintf("S %d", i), fmt.Sprintf("s%04d@bitsathy.ac.in", i), "IT")
	}
	result, err := repo.ImportStudents(context.Background(), rows)
	if err != nil || result.Created != len(rows) {
		t.Fatalf("ImportStudents() = %+v, %v", result, err)
	}
	var mismatched int
	err = db.QueryRow(`SELECT COUNT(*) FROM users u JOIN student_profiles sp ON sp.user_id = u.id
        WHERE CONCAT('s', SUBSTRING(sp.register_number, 2), '@bitsathy.ac.in') <> u.email`).Scan(&mismatched)
	if err != nil || mismatched != 0 {
		t.Fatalf("%d profiles attached to the wrong user, %v", mismatched, err)
	}
}

func TestCreateAdminAndList(t *testing.T) {
	db := dbtest.Open(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	insertUserWithRole(t, db, "pupil@bitsathy.ac.in", "student")

	admin, err := repo.CreateAdmin(ctx, users.AdminInput{Name: "Dr Staff", Email: "staff@bitsathy.ac.in"})
	if err != nil || admin.ID == 0 || admin.Name != "Dr Staff" || admin.Status != "active" || len(admin.PublicID) != 26 {
		t.Fatalf("CreateAdmin() = %+v, %v", admin, err)
	}
	for _, email := range []string{"staff@bitsathy.ac.in", "STAFF@bitsathy.ac.in", "pupil@bitsathy.ac.in"} {
		if _, err := repo.CreateAdmin(ctx, users.AdminInput{Name: "Again", Email: email}); !errors.Is(err, users.ErrConflict) {
			t.Errorf("CreateAdmin(%s) error = %v; want ErrConflict", email, err)
		}
	}

	admins, err := repo.Admins(ctx)
	if err != nil || len(admins) != 1 || admins[0].Email != "staff@bitsathy.ac.in" {
		t.Fatalf("Admins() = %+v, %v", admins, err)
	}
}
