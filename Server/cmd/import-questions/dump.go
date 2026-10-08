package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// value is one field of a dumped row; null distinguishes SQL NULL from an
// empty string.
type value struct {
	text string
	null bool
}

// readDump returns the rows of every INSERT statement in a mysqldump file,
// keyed by table name. mysqldump writes one INSERT per line, so the file is
// read line by line; lines can be several megabytes long.
func readDump(r io.Reader) (map[string][][]value, error) {
	tables := make(map[string][][]value)
	reader := bufio.NewReader(r)
	for lineNo := 1; ; lineNo++ {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if table, rest, ok := insertStatement(line); ok {
			rows, parseErr := parseRows(rest)
			if parseErr != nil {
				return nil, fmt.Errorf("line %d (%s): %w", lineNo, table, parseErr)
			}
			tables[table] = append(tables[table], rows...)
		}
		if errors.Is(err, io.EOF) {
			return tables, nil
		}
	}
}

func insertStatement(line string) (table, rest string, ok bool) {
	const prefix = "INSERT INTO `"
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	line = line[len(prefix):]
	end := strings.IndexByte(line, '`')
	if end < 0 {
		return "", "", false
	}
	table, line = line[:end], line[end+1:]
	line, ok = strings.CutPrefix(line, " VALUES ")
	return table, strings.TrimRight(line, "\r\n"), ok
}

// parseRows parses "(v,v,...),(v,...);" as mysqldump writes it.
func parseRows(s string) ([][]value, error) {
	p := &rowParser{s: s}
	var rows [][]value
	for {
		row, err := p.row()
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)

		switch p.next() {
		case ',':
			continue
		case ';':
			if p.pos != len(p.s) {
				return nil, fmt.Errorf("unexpected text after ';' at %d", p.pos)
			}
			return rows, nil
		default:
			return nil, fmt.Errorf("expected ',' or ';' at %d", p.pos-1)
		}
	}
}

type rowParser struct {
	s   string
	pos int
}

func (p *rowParser) next() byte {
	if p.pos >= len(p.s) {
		return 0
	}
	c := p.s[p.pos]
	p.pos++
	return c
}

func (p *rowParser) row() ([]value, error) {
	if p.next() != '(' {
		return nil, fmt.Errorf("expected '(' at %d", p.pos-1)
	}
	var row []value
	for {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		row = append(row, v)

		switch p.next() {
		case ',':
			continue
		case ')':
			return row, nil
		default:
			return nil, fmt.Errorf("expected ',' or ')' at %d", p.pos-1)
		}
	}
}

func (p *rowParser) value() (value, error) {
	if p.pos < len(p.s) && p.s[p.pos] == '\'' {
		p.pos++
		return p.quoted()
	}
	start := p.pos
	for p.pos < len(p.s) && p.s[p.pos] != ',' && p.s[p.pos] != ')' {
		p.pos++
	}
	raw := p.s[start:p.pos]
	if raw == "" {
		return value{}, fmt.Errorf("empty value at %d", start)
	}
	if raw == "NULL" {
		return value{null: true}, nil
	}
	return value{text: raw}, nil
}

// quoted reads a single-quoted string up to its closing quote, undoing
// mysqldump's backslash escapes.
func (p *rowParser) quoted() (value, error) {
	var b strings.Builder
	for p.pos < len(p.s) {
		c := p.next()
		switch c {
		case '\'':
			return value{text: b.String()}, nil
		case '\\':
			e := p.next()
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '0':
				b.WriteByte(0)
			case 'Z':
				b.WriteByte(26)
			case 0:
				return value{}, errors.New("unterminated escape")
			default: // \\ \' \" and anything else stand for themselves
				b.WriteByte(e)
			}
		default:
			b.WriteByte(c)
		}
	}
	return value{}, errors.New("unterminated string")
}
