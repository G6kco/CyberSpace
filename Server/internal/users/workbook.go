package users

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// Limits on an uploaded workbook. The unzipped limit stops a small file that
// expands enormously, and bounds memory: excelize holds parts like the
// shared strings in memory. The reference list (four year groups, ~7,400
// students) is under 2 MiB unzipped per sheet.
const (
	maxUnzippedBytes = 64 << 20
	maxStudentRows   = 50000
	headerSearchRows = 10
)

// Columns are found by header, not position, because each year's sheet
// names them differently ("Reg No", "Reg. No.", "Roll No"; "Email ID",
// "Student Official Email ID"; "Dept").
type column int

const (
	colRegister column = iota
	colName
	colEmail
	colDepartment
	columnCount
)

func headerColumn(cell string) (column, bool) {
	key := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, cell)
	switch key {
	case "regno", "registerno", "registrationno", "registernumber", "registrationnumber", "rollno", "rollnumber":
		return colRegister, true
	case "name", "studentname", "fullname":
		return colName, true
	case "dept", "department", "branch":
		return colDepartment, true
	}
	if strings.Contains(key, "email") || key == "mail" || key == "mailid" {
		return colEmail, true
	}
	return 0, false
}

// rawRow is one data row as read, before validation.
type rawRow struct {
	sheet  string
	row    int
	fields [columnCount]string
}

// readWorkbook returns the data rows of every sheet that has the four
// columns, and an issue for each sheet that does not.
func readWorkbook(r io.Reader) ([]rawRow, []RowIssue, error) {
	// Read first, so an upload over the size limit is reported as too large
	// rather than as an unreadable workbook.
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	book, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: maxUnzippedBytes})
	if err != nil {
		return nil, nil, fmt.Errorf("%w: the file is not a readable .xlsx workbook", ErrInvalidInput)
	}
	defer book.Close()

	var rows []rawRow
	var issues []RowIssue
	for _, sheet := range book.GetSheetList() {
		sheetRows, found, err := readSheet(book, sheet, maxStudentRows-len(rows))
		if err != nil {
			return nil, nil, err
		}
		if !found {
			issues = append(issues, RowIssue{Sheet: sheet,
				Reason: "No header row naming the register number, student name, email and department; sheet skipped"})
			continue
		}
		rows = append(rows, sheetRows...)
	}
	if len(rows) == 0 && len(issues) == len(book.GetSheetList()) {
		return nil, nil, fmt.Errorf("%w: no sheet has a header row with Reg No, Student Name, Email and Department columns", ErrInvalidInput)
	}
	return rows, issues, nil
}

func readSheet(book *excelize.File, sheet string, room int) ([]rawRow, bool, error) {
	iter, err := book.Rows(sheet)
	if err != nil {
		return nil, false, fmt.Errorf("%w: sheet %q cannot be read", ErrInvalidInput, sheet)
	}
	defer iter.Close()

	var index [columnCount]int
	found := false
	var rows []rawRow
	for n := 1; iter.Next(); n++ {
		cells, err := iter.Columns()
		if err != nil {
			return nil, false, fmt.Errorf("%w: sheet %q row %d cannot be read", ErrInvalidInput, sheet, n)
		}
		if !found {
			if n > headerSearchRows {
				return nil, false, nil
			}
			index, found = headerIndex(cells)
			continue
		}
		var row rawRow
		blank := true
		for c := range columnCount {
			if i := index[c]; i < len(cells) {
				row.fields[c] = cells[i]
				blank = blank && strings.TrimSpace(cells[i]) == ""
			}
		}
		if blank {
			continue
		}
		if len(rows) >= room {
			return nil, false, fmt.Errorf("%w: the workbook has more than %d students; split it into several uploads", ErrInvalidInput, maxStudentRows)
		}
		row.sheet, row.row = sheet, n
		rows = append(rows, row)
	}
	if err := iter.Error(); err != nil {
		return nil, false, fmt.Errorf("%w: sheet %q cannot be read", ErrInvalidInput, sheet)
	}
	return rows, found, nil
}

// headerIndex finds the four columns in a row; the first match of each wins.
func headerIndex(cells []string) ([columnCount]int, bool) {
	var index [columnCount]int
	var seen [columnCount]bool
	for i, cell := range cells {
		if c, ok := headerColumn(cell); ok && !seen[c] {
			index[c], seen[c] = i, true
		}
	}
	return index, seen == [columnCount]bool{true, true, true, true}
}
