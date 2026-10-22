package students

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Record struct {
	RegisterNumber string
	DisplayName    string
	Email          string
	Department     string
	Programme      string
	GraduationYear int
}

type Result struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

var sheetYear = map[string]int{
	"IV Name List":  2026,
	"III Name List": 2027,
	"II Name List":  2028,
	"I Name List":   2029,
}

var btech = map[string]struct{}{
	"AD": {}, "AM": {}, "ARTIFICIAL INTELLIGENCE AND DATA SCIENCE": {},
	"ARTIFICIAL INTELLIGENCE AND MACHINE LEARNING": {}, "BIOTECHNOLOGY": {},
	"BT": {}, "COMPUTER SCIENCE AND BUSINESS SYSTEMS": {}, "CSBS": {},
	"COMPUTER TECHNOLOGY": {}, "FOOD TECHNOLOGY": {}, "FASHION TECHNOLOGY": {},
	"TEXTILE TECHNOLOGY": {}, "INFORMATION TECHNOLOGY": {}, "IT": {},
	"INFORMATION SCIENCE AND ENGINEERING": {},
}

func programme(dept string) string {
	if _, ok := btech[strings.ToUpper(strings.TrimSpace(dept))]; ok {
		return "B.Tech"
	}
	return "B.E."
}

func yearFor(sheet, reg string) int {
	if y, ok := sheetYear[sheet]; ok {
		return y
	}
	if len(reg) >= 7 && strings.HasPrefix(reg, "73762") {
		if yy, err := strconv.Atoi(reg[5:7]); err == nil {
			return 2000 + yy + 4
		}
	}
	if len(reg) >= 4 {
		if yyyy, err := strconv.Atoi(reg[:4]); err == nil && yyyy >= 2000 && yyyy < 2100 {
			return yyyy + 4
		}
	}
	return 0
}

func ParseExcel(path string) ([]Record, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	seen := make(map[string]struct{}, 8000)
	out := make([]Record, 0, 8000)

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("read sheet %s: %w", sheet, err)
		}
		if len(rows) == 0 {
			continue
		}
		col := map[string]int{}
		for i, h := range rows[0] {
			col[strings.ToLower(strings.TrimSpace(h))] = i
		}
		get := func(row []string, names ...string) string {
			for _, n := range names {
				if i, ok := col[n]; ok && i < len(row) {
					return strings.TrimSpace(row[i])
				}
			}
			return ""
		}
		for _, row := range rows[1:] {
			email := strings.ToLower(get(row, "email", "student official email id", "email id"))
			reg := get(row, "register_number", "register number", "reg no", "reg. no.", "roll no")
			name := get(row, "display_name", "display name", "student name", "name")
			if email == "" || reg == "" || name == "" {
				continue
			}
			if _, dup := seen[email]; dup {
				continue
			}
			seen[email] = struct{}{}
			dept := get(row, "department", "dept")
			out = append(out, Record{
				RegisterNumber: reg,
				DisplayName:    name,
				Email:          email,
				Department:     dept,
				Programme:      programme(dept),
				GraduationYear: yearFor(sheet, reg),
			})
		}
	}
	return out, nil
}

func Import(ctx context.Context, db *sql.DB, records []Record) (Result, error) {
	var res Result
	if len(records) == 0 {
		return res, nil
	}

	byYear := map[int][]Record{}
	var years []int
	for _, r := range records {
		if _, ok := byYear[r.GraduationYear]; !ok {
			years = append(years, r.GraduationYear)
		}
		byYear[r.GraduationYear] = append(byYear[r.GraduationYear], r)
	}
	sort.Ints(years)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	for _, y := range years {
		chunk := byYear[y]
		query := "INSERT IGNORE INTO users (public_id, email, display_name, role, status) VALUES " +
			strings.Repeat("(UPPER(LEFT(REPLACE(UUID(), '-', ''), 26)), ?, ?, 'student', 'active'),", len(chunk))
		query = strings.TrimSuffix(query, ",")
		args := make([]any, 0, len(chunk)*2)
		for _, r := range chunk {
			args = append(args, r.Email, r.DisplayName)
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return res, fmt.Errorf("insert users batch %d: %w", y, err)
		}
		for _, r := range chunk {
			out, err := tx.ExecContext(ctx,
				`INSERT IGNORE INTO student_profiles
				(user_id, register_number, department, programme, graduation_year)
				SELECT id, ?, ?, ?, ? FROM users WHERE email = ?`,
				r.RegisterNumber, r.Department, r.Programme, r.GraduationYear, r.Email,
			)
			if err != nil {
				return res, fmt.Errorf("insert profile %s: %w", r.Email, err)
			}
			if n, _ := out.RowsAffected(); n == 1 {
				res.Imported++
			} else {
				res.Skipped++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}
