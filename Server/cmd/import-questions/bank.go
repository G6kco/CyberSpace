package main

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
)

// questionPoints is what every bank question is worth; an attempt deals one
// question per pool, so it is out of 100.
const questionPoints = 50

// question is one bank question with its flags, in import order.
type question struct {
	dumpID     int
	pool       string // "wireshark" or "nmap"
	difficulty string // "easy" or "hard"
	prompt     string
	pcapFile   string // Wireshark only: bare file name
	image      string // Nmap only: Docker image
	flags      []answer
}

// answer is one flag the student submits. The first flag of a question is
// its main flag; later ones are its sub-questions.
type answer struct {
	prompt string
	answer string
	points int
}

// buildBank turns the reference dump's questions, questions_flags,
// subquestions and subquestions_flags tables into questions, checking every
// question is complete and worth questionPoints.
func buildBank(tables map[string][][]value) ([]question, error) {
	flagsByQuestion, err := groupFlags(tables["questions_flags"], "questions_flags")
	if err != nil {
		return nil, err
	}
	subflags, err := groupFlags(tables["subquestions_flags"], "subquestions_flags")
	if err != nil {
		return nil, err
	}

	type sub struct {
		order int
		flag  answer
	}
	subsByQuestion := map[int][]sub{}
	for _, row := range tables["subquestions"] {
		// (id, question_id, question_text, order_index)
		if len(row) != 4 {
			return nil, fmt.Errorf("subquestions: row has %d fields; want 4", len(row))
		}
		id, questionID, order, err := ints(row[0], row[1], row[3])
		if err != nil {
			return nil, fmt.Errorf("subquestions: %w", err)
		}
		answers := subflags[id]
		if len(answers) != 1 {
			return nil, fmt.Errorf("subquestion %d has %d flags; want 1", id, len(answers))
		}
		answers[0].prompt = strings.TrimSpace(row[2].text)
		subsByQuestion[questionID] = append(subsByQuestion[questionID], sub{order: order, flag: answers[0]})
	}

	var bank []question
	for _, row := range tables["questions"] {
		// (id, type, title, description, pcap_file, docker_image_name,
		//  difficulty, total_marks, created_at)
		if len(row) != 9 {
			return nil, fmt.Errorf("questions: row has %d fields; want 9", len(row))
		}
		id, err := strconv.Atoi(row[0].text)
		if err != nil {
			return nil, fmt.Errorf("questions: id %q: %w", row[0].text, err)
		}
		q := question{
			dumpID:     id,
			pool:       row[1].text,
			difficulty: strings.ToLower(strings.TrimSpace(row[6].text)),
			prompt:     strings.TrimSpace(row[3].text),
		}

		mainFlags := flagsByQuestion[id]
		if len(mainFlags) != 1 {
			return nil, fmt.Errorf("question %d has %d main flags; want 1", id, len(mainFlags))
		}
		mainFlags[0].prompt = "Submit the flag."
		q.flags = append(q.flags, mainFlags[0])

		subs := subsByQuestion[id]
		slices.SortFunc(subs, func(a, b sub) int { return a.order - b.order })
		for _, s := range subs {
			q.flags = append(q.flags, s.flag)
		}

		switch q.pool {
		case "wireshark":
			// The dump stores "PCAP/scenario2.pcap"; the API serves bare
			// names from its own capture directory.
			q.pcapFile = path.Base(strings.TrimSpace(row[4].text))
			if q.pcapFile == "" || q.pcapFile == "." || q.pcapFile == "/" {
				return nil, fmt.Errorf("wireshark question %d has no pcap file", id)
			}
		case "nmap":
			q.image = strings.ToLower(strings.TrimSpace(row[5].text))
			if q.image == "" {
				return nil, fmt.Errorf("nmap question %d has no docker image", id)
			}
		default:
			return nil, fmt.Errorf("question %d has unknown type %q", id, q.pool)
		}
		if q.difficulty != "easy" && q.difficulty != "hard" {
			return nil, fmt.Errorf("question %d has difficulty %q; want easy or hard", id, q.difficulty)
		}

		total := 0
		for _, f := range q.flags {
			total += f.points
		}
		if total != questionPoints {
			return nil, fmt.Errorf("question %d flags total %d points; want %d", id, total, questionPoints)
		}
		bank = append(bank, q)
	}

	if len(bank) == 0 {
		return nil, errors.New("dump contains no questions")
	}
	slices.SortFunc(bank, func(a, b question) int { return a.dumpID - b.dumpID })
	return bank, nil
}

// groupFlags reads (id, owner_id, flag, marks) rows keyed by owner_id. The
// dump's column is named flag_hash but holds the plaintext answer.
func groupFlags(rows [][]value, table string) (map[int][]answer, error) {
	byOwner := map[int][]answer{}
	for _, row := range rows {
		if len(row) != 4 {
			return nil, fmt.Errorf("%s: row has %d fields; want 4", table, len(row))
		}
		owner, points, err := ints2(row[1], row[3])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", table, err)
		}
		text := strings.TrimSpace(row[2].text)
		if text == "" || points <= 0 {
			return nil, fmt.Errorf("%s: row %s has an empty answer or no points", table, row[0].text)
		}
		byOwner[owner] = append(byOwner[owner], answer{answer: text, points: points})
	}
	return byOwner, nil
}

func ints(a, b, c value) (int, int, int, error) {
	x, y, err := ints2(a, b)
	if err != nil {
		return 0, 0, 0, err
	}
	z, err := strconv.Atoi(c.text)
	return x, y, z, err
}

func ints2(a, b value) (int, int, error) {
	x, err := strconv.Atoi(a.text)
	if err != nil {
		return 0, 0, err
	}
	y, err := strconv.Atoi(b.text)
	return x, y, err
}
