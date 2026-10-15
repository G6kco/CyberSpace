package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/G6kco/CyberSpace/internal/assessments"
	"github.com/G6kco/CyberSpace/internal/labs"
	"github.com/G6kco/CyberSpace/internal/levels"
)

// LevelRepository stores levels built by administrators. A built level is
// one levels row, one assessments row and one assessment_versions row with
// questions_per_attempt set; its questions are 'scenario' groups whose
// children are the answers, each with one case-insensitive HMAC validator.
type LevelRepository struct {
	db *sql.DB
}

func NewLevelRepository(db *sql.DB) *LevelRepository {
	return &LevelRepository{db: db}
}

// levelColumns and levelFrom read a level through its first assessment and
// that assessment's newest version. Scanned by scanLevel.
const levelColumns = `l.id, l.sequence_no, l.name, a.title, v.instructions, v.duration_seconds,
    COALESCE(v.questions_per_attempt, 0), v.pass_percent, v.pass_score, v.total_points,
    v.status,
    (SELECT COUNT(*) FROM assessment_questions q
     WHERE q.assessment_version_id = v.id AND q.parent_question_id IS NULL),
    (SELECT COUNT(*) FROM attempts t
     JOIN assessment_access_grants g ON g.id = t.access_grant_id
     WHERE g.assessment_version_id = v.id)`

const levelFrom = `
    FROM levels l
    JOIN assessments a ON a.id = (SELECT MIN(a2.id) FROM assessments a2 WHERE a2.level_id = l.id)
    JOIN assessment_versions v ON v.id = (
        SELECT MAX(v2.id) FROM assessment_versions v2 WHERE v2.assessment_id = a.id
    )`

func scanLevel(row rowScanner) (levels.Level, error) {
	var l levels.Level
	var durationSeconds int
	var passPercent sql.NullFloat64
	var passScore, total float64
	if err := row.Scan(&l.ID, &l.Sequence, &l.Name, &l.Title, &l.Instructions, &durationSeconds,
		&l.QuestionsPerAttempt, &passPercent, &passScore, &total, &l.Status,
		&l.QuestionCount, &l.Attempts); err != nil {
		return l, err
	}
	l.DurationMinutes = durationSeconds / 60
	l.Built = l.QuestionsPerAttempt > 0
	switch {
	case passPercent.Valid:
		l.PassPercent = passPercent.Float64
	case total > 0:
		l.PassPercent = math.Round(passScore/total*10000) / 100
	}
	if l.Status != levels.StatusPublished {
		l.Status = levels.StatusDraft
	}
	return l, nil
}

func (r *LevelRepository) List(ctx context.Context) ([]levels.Level, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+levelColumns+levelFrom+` ORDER BY l.sequence_no`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []levels.Level{}
	for rows.Next() {
		l, err := scanLevel(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

func (r *LevelRepository) Get(ctx context.Context, levelID uint64) (levels.Level, error) {
	var versionID uint64
	l, err := scanLevel(r.db.QueryRowContext(ctx,
		`SELECT `+levelColumns+levelFrom+` WHERE l.id = ?`, levelID))
	if errors.Is(err, sql.ErrNoRows) {
		return levels.Level{}, levels.ErrNotFound
	}
	if err != nil {
		return levels.Level{}, err
	}
	if err := r.db.QueryRowContext(ctx,
		`SELECT v.id`+levelFrom+` WHERE l.id = ?`, levelID).Scan(&versionID); err != nil {
		return levels.Level{}, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT q.id, q.code, q.prompt, COALESCE(ltv.yaml_spec, '')
        FROM assessment_questions q
        LEFT JOIN lab_template_versions ltv ON ltv.id = q.lab_template_version_id
        WHERE q.assessment_version_id = ? AND q.parent_question_id IS NULL
        ORDER BY q.sequence_no, q.id`,
		versionID)
	if err != nil {
		return levels.Level{}, err
	}
	index := map[uint64]int{}
	l.Questions = []levels.Question{}
	for rows.Next() {
		var q levels.Question
		var spec string
		if err := rows.Scan(&q.ID, &q.Code, &q.Prompt, &spec); err != nil {
			rows.Close()
			return levels.Level{}, err
		}
		if spec != "" {
			if parsed, err := labs.ParseSpec(spec); err == nil {
				q.Image = parsed.Image
			}
		}
		q.Answers, q.Files = []levels.Answer{}, []levels.File{}
		index[q.ID] = len(l.Questions)
		l.Questions = append(l.Questions, q)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return levels.Level{}, err
	}

	answers, err := r.db.QueryContext(ctx,
		`SELECT parent_question_id, id, prompt, points FROM assessment_questions
        WHERE assessment_version_id = ? AND parent_question_id IS NOT NULL
        ORDER BY parent_question_id, sequence_no, id`,
		versionID)
	if err != nil {
		return levels.Level{}, err
	}
	for answers.Next() {
		var parentID uint64
		var a levels.Answer
		if err := answers.Scan(&parentID, &a.ID, &a.Prompt, &a.Marks); err != nil {
			answers.Close()
			return levels.Level{}, err
		}
		if i, ok := index[parentID]; ok {
			l.Questions[i].Answers = append(l.Questions[i].Answers, a)
			l.Questions[i].Marks += a.Marks
		}
	}
	answers.Close()
	if err := answers.Err(); err != nil {
		return levels.Level{}, err
	}

	files, err := r.db.QueryContext(ctx,
		`SELECT f.question_id, f.id, f.original_name, f.size_bytes, f.stored_name
        FROM question_files f
        JOIN assessment_questions q ON q.id = f.question_id
        WHERE q.assessment_version_id = ?
        ORDER BY f.id`,
		versionID)
	if err != nil {
		return levels.Level{}, err
	}
	defer files.Close()
	for files.Next() {
		var questionID uint64
		var f levels.File
		if err := files.Scan(&questionID, &f.ID, &f.Name, &f.Size, &f.StoredName); err != nil {
			return levels.Level{}, err
		}
		if i, ok := index[questionID]; ok {
			l.Questions[i].Files = append(l.Questions[i].Files, f)
		}
	}
	return l, files.Err()
}

func (r *LevelRepository) Create(ctx context.Context, adminID uint64, in levels.LevelInput) (uint64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// A new level comes after every existing one. The unique sequence_no
	// rejects a concurrent create that read the same maximum.
	var sequence int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sequence_no), 0) + 1 FROM levels`).Scan(&sequence); err != nil {
		return 0, err
	}
	slug, err := freeSlug(ctx, tx, in.Name)
	if err != nil {
		return 0, err
	}

	levelID, err := insertedID(tx.ExecContext(ctx,
		`INSERT INTO levels (name, slug, sequence_no, status) VALUES (?, ?, ?, 'draft')`,
		in.Name, slug, sequence))
	if err != nil {
		return 0, err
	}
	assessmentID, err := insertedID(tx.ExecContext(ctx,
		`INSERT INTO assessments (level_id, title, slug, created_by) VALUES (?, ?, ?, ?)`,
		levelID, in.Title, slug, adminID))
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO assessment_versions
        (assessment_id, version_no, instructions, duration_seconds, pass_score, pass_percent,
         total_points, max_attempts, questions_per_attempt, status, created_by)
        VALUES (?, 1, ?, ?, 0, ?, 0, 1, ?, 'draft', ?)`,
		assessmentID, in.Instructions, in.DurationMinutes*60, in.PassPercent,
		in.QuestionsPerAttempt, adminID); err != nil {
		return 0, err
	}
	return levelID, tx.Commit()
}

func (r *LevelRepository) Update(ctx context.Context, levelID uint64, in levels.LevelInput) error {
	return r.inDraft(ctx, levelID, func(tx *sql.Tx, d draft) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE levels SET name = ? WHERE id = ?`, in.Name, levelID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE assessments SET title = ? WHERE id = ?`, in.Title, d.assessmentID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE assessment_versions
            SET instructions = ?, duration_seconds = ?, questions_per_attempt = ?, pass_percent = ?
            WHERE id = ?`,
			in.Instructions, in.DurationMinutes*60, in.QuestionsPerAttempt, in.PassPercent, d.versionID)
		return err
	})
}

func (r *LevelRepository) Delete(ctx context.Context, levelID uint64) ([]string, error) {
	var stored []string
	err := r.inDraft(ctx, levelID, func(tx *sql.Tx, d draft) error {
		// Grants keep the version for the record, even revoked ones.
		var grants int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM assessment_access_grants WHERE assessment_version_id = ?`,
			d.versionID).Scan(&grants); err != nil {
			return err
		}
		if grants > 0 {
			return levels.ErrInUse
		}
		var err error
		if stored, err = storedNames(ctx, tx,
			`SELECT f.stored_name FROM question_files f
            JOIN assessment_questions q ON q.id = f.question_id
            WHERE q.assessment_version_id = ?`, d.versionID); err != nil {
			return err
		}
		// Questions, answers, validators and files go with the version.
		for _, statement := range []struct {
			query string
			id    uint64
		}{
			{`DELETE FROM assessment_versions WHERE id = ?`, d.versionID},
			{`DELETE FROM assessments WHERE id = ?`, d.assessmentID},
			{`DELETE FROM levels WHERE id = ?`, levelID},
		} {
			if _, err := tx.ExecContext(ctx, statement.query, statement.id); err != nil {
				return err
			}
		}
		return nil
	})
	return stored, err
}

func (r *LevelRepository) Publish(ctx context.Context, levelID uint64, now time.Time) error {
	return r.inDraft(ctx, levelID, func(tx *sql.Tx, d draft) error {
		var questions, unanswered int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*),
                    COALESCE(SUM(NOT EXISTS (
                        SELECT 1 FROM assessment_questions c WHERE c.parent_question_id = q.id
                    )), 0)
            FROM assessment_questions q
            WHERE q.assessment_version_id = ? AND q.parent_question_id IS NULL`,
			d.versionID).Scan(&questions, &unanswered); err != nil {
			return err
		}
		switch {
		case questions < d.perAttempt:
			return fmt.Errorf("%w: each student is dealt %d questions but the level has %d",
				levels.ErrNotReady, d.perAttempt, questions)
		case unanswered > 0:
			return fmt.Errorf("%w: %d question(s) have no answers", levels.ErrNotReady, unanswered)
		}

		// The version's total is the highest maximum a student can be
		// dealt; each attempt stores its own maximum and pass mark.
		var total float64
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(marks), 0) FROM (
                SELECT (SELECT SUM(c.points) FROM assessment_questions c
                        WHERE c.parent_question_id = q.id) AS marks
                FROM assessment_questions q
                WHERE q.assessment_version_id = ? AND q.parent_question_id IS NULL
                ORDER BY marks DESC
                LIMIT ?
            ) top`,
			d.versionID, d.perAttempt).Scan(&total); err != nil {
			return err
		}
		passScore := math.Round(total*d.passPercent) / 100

		if _, err := tx.ExecContext(ctx,
			`UPDATE assessment_versions
            SET total_points = ?, pass_score = ?, status = 'published', published_at = ?
            WHERE id = ?`,
			total, passScore, now, d.versionID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE assessments SET status = 'published', current_version_id = ? WHERE id = ?`,
			d.versionID, d.assessmentID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE levels SET status = 'published' WHERE id = ?`, levelID)
		return err
	})
}

func (r *LevelRepository) Unpublish(ctx context.Context, adminID, levelID uint64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	d, err := lockLevel(ctx, tx, levelID)
	if err != nil {
		return err
	}
	if d.status != levels.StatusPublished {
		return fmt.Errorf("%w: the level is not published", levels.ErrInvalidInput)
	}
	var attempts int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM attempts t
        JOIN assessment_access_grants g ON g.id = t.access_grant_id
        WHERE g.assessment_version_id = ?`,
		d.versionID).Scan(&attempts); err != nil {
		return err
	}
	if attempts > 0 {
		return levels.ErrInUse
	}

	// Attendance taken for the level must not start it once republished.
	if _, err := tx.ExecContext(ctx,
		`UPDATE assessment_access_grants
        SET status = 'revoked', revoked_by = ?, revoked_at = ?, revocation_reason = 'Level unpublished'
        WHERE assessment_version_id = ? AND status = 'approved'`,
		adminID, now, d.versionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE assessments SET status = 'draft', current_version_id = NULL WHERE id = ?`,
		d.assessmentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE assessment_versions SET status = 'draft', published_at = NULL WHERE id = ?`,
		d.versionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE levels SET status = 'draft' WHERE id = ?`, levelID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *LevelRepository) CreateQuestion(ctx context.Context, adminID, levelID uint64, in levels.QuestionInput) (uint64, error) {
	var questionID uint64
	err := r.inDraft(ctx, levelID, func(tx *sql.Tx, d draft) error {
		labID, err := labFor(ctx, tx, in.Image, adminID)
		if err != nil {
			return err
		}
		var sequence int
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(sequence_no), 0) + 1 FROM assessment_questions
            WHERE assessment_version_id = ? AND parent_question_id IS NULL`,
			d.versionID).Scan(&sequence); err != nil {
			return err
		}
		code := fmt.Sprintf("q-%02d", sequence)
		questionID, err = insertedID(tx.ExecContext(ctx,
			`INSERT INTO assessment_questions
            (assessment_version_id, code, prompt, answer_type, pool,
             lab_template_version_id, points, sequence_no)
            VALUES (?, ?, ?, 'group', ?, ?, 0, ?)`,
			d.versionID, code, in.Prompt, assessments.PoolScenario, labID, sequence))
		if err != nil {
			return err
		}
		return saveAnswers(ctx, tx, d.versionID, questionID, code, in.Answers)
	})
	return questionID, err
}

func (r *LevelRepository) UpdateQuestion(ctx context.Context, adminID, levelID, questionID uint64, in levels.QuestionInput) error {
	return r.inDraft(ctx, levelID, func(tx *sql.Tx, d draft) error {
		code, err := questionCode(ctx, tx, d.versionID, questionID)
		if err != nil {
			return err
		}
		labID, err := labFor(ctx, tx, in.Image, adminID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE assessment_questions SET prompt = ?, lab_template_version_id = ? WHERE id = ?`,
			in.Prompt, labID, questionID); err != nil {
			return err
		}
		return saveAnswers(ctx, tx, d.versionID, questionID, code, in.Answers)
	})
}

func (r *LevelRepository) DeleteQuestion(ctx context.Context, levelID, questionID uint64) ([]string, error) {
	var stored []string
	err := r.inDraft(ctx, levelID, func(tx *sql.Tx, d draft) error {
		if _, err := questionCode(ctx, tx, d.versionID, questionID); err != nil {
			return err
		}
		var err error
		if stored, err = storedNames(ctx, tx,
			`SELECT stored_name FROM question_files WHERE question_id = ?`, questionID); err != nil {
			return err
		}
		// Answers, their validators and the files' rows cascade.
		_, err = tx.ExecContext(ctx, `DELETE FROM assessment_questions WHERE id = ?`, questionID)
		return err
	})
	return stored, err
}

func (r *LevelRepository) AddFile(ctx context.Context, adminID, levelID uint64, f levels.NewFile) (uint64, error) {
	var fileID uint64
	err := r.inDraft(ctx, levelID, func(tx *sql.Tx, d draft) error {
		if _, err := questionCode(ctx, tx, d.versionID, f.QuestionID); err != nil {
			return err
		}
		var err error
		fileID, err = insertedID(tx.ExecContext(ctx,
			`INSERT INTO question_files
            (question_id, stored_name, original_name, size_bytes, sha256, created_by)
            VALUES (?, ?, ?, ?, ?, ?)`,
			f.QuestionID, f.StoredName, f.OriginalName, f.Size, f.SHA256[:], adminID))
		return err
	})
	return fileID, err
}

func (r *LevelRepository) DeleteFile(ctx context.Context, levelID, questionID, fileID uint64) (string, error) {
	var stored string
	err := r.inDraft(ctx, levelID, func(tx *sql.Tx, d draft) error {
		f, err := fileOf(ctx, tx, d.versionID, questionID, fileID)
		if err != nil {
			return err
		}
		stored = f.StoredName
		_, err = tx.ExecContext(ctx, `DELETE FROM question_files WHERE id = ?`, fileID)
		return err
	})
	return stored, err
}

func (r *LevelRepository) File(ctx context.Context, levelID, questionID, fileID uint64) (levels.File, error) {
	var versionID uint64
	err := r.db.QueryRowContext(ctx, `SELECT v.id`+levelFrom+` WHERE l.id = ?`, levelID).Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		return levels.File{}, levels.ErrNotFound
	}
	if err != nil {
		return levels.File{}, err
	}
	return fileOf(ctx, r.db, versionID, questionID, fileID)
}

// ---- Helpers ---------------------------------------------------------------

// draft is a built level's rows, locked for a change.
type draft struct {
	assessmentID uint64
	versionID    uint64
	status       string
	perAttempt   int
	passPercent  float64
}

// lockLevel locks a built level's version row. Level 1 is refused: it was
// imported, deals by pool, and is not edited here.
func lockLevel(ctx context.Context, tx *sql.Tx, levelID uint64) (draft, error) {
	var d draft
	var perAttempt sql.NullInt64
	var passPercent sql.NullFloat64
	err := tx.QueryRowContext(ctx,
		`SELECT a.id, v.id, v.status, v.questions_per_attempt, v.pass_percent`+levelFrom+`
        WHERE l.id = ?
        FOR UPDATE`,
		levelID).Scan(&d.assessmentID, &d.versionID, &d.status, &perAttempt, &passPercent)
	if errors.Is(err, sql.ErrNoRows) {
		return d, levels.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	if !perAttempt.Valid {
		return d, levels.ErrReadOnly
	}
	d.perAttempt, d.passPercent = int(perAttempt.Int64), passPercent.Float64
	return d, nil
}

// inDraft runs change in one transaction on a built level that is a draft.
func (r *LevelRepository) inDraft(ctx context.Context, levelID uint64, change func(*sql.Tx, draft) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	d, err := lockLevel(ctx, tx, levelID)
	if err != nil {
		return err
	}
	if d.status != levels.StatusDraft {
		return levels.ErrPublished
	}
	if err := change(tx, d); err != nil {
		return err
	}
	return tx.Commit()
}

// questionCode returns the code of a top-level question of the version.
func questionCode(ctx context.Context, tx *sql.Tx, versionID, questionID uint64) (string, error) {
	var code string
	err := tx.QueryRowContext(ctx,
		`SELECT code FROM assessment_questions
        WHERE id = ? AND assessment_version_id = ? AND parent_question_id IS NULL`,
		questionID, versionID).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", levels.ErrNotFound
	}
	return code, err
}

// saveAnswers makes the question's answers exactly those given: listed
// existing answers are updated (their digest only when a new answer was
// entered), unlisted ones are deleted, and new ones are added.
func saveAnswers(
	ctx context.Context,
	tx *sql.Tx,
	versionID, questionID uint64,
	code string,
	answers []levels.AnswerInput,
) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, code FROM assessment_questions WHERE parent_question_id = ?`, questionID)
	if err != nil {
		return err
	}
	existing := map[uint64]string{}
	used := map[string]bool{}
	for rows.Next() {
		var id uint64
		var childCode string
		if err := rows.Scan(&id, &childCode); err != nil {
			rows.Close()
			return err
		}
		existing[id] = childCode
		used[childCode] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	keep := map[uint64]bool{}
	for _, a := range answers {
		if a.ID == 0 {
			continue
		}
		if _, ok := existing[a.ID]; !ok {
			return fmt.Errorf("%w: answer %d does not belong to this question", levels.ErrInvalidInput, a.ID)
		}
		keep[a.ID] = true
	}
	for id := range existing {
		if !keep[id] {
			if _, err := tx.ExecContext(ctx, `DELETE FROM assessment_questions WHERE id = ?`, id); err != nil {
				return err
			}
		}
	}

	next := 1
	for i, a := range answers {
		sequence := i + 1
		if a.ID != 0 {
			if _, err := tx.ExecContext(ctx,
				`UPDATE assessment_questions SET prompt = ?, points = ?, sequence_no = ? WHERE id = ?`,
				a.Prompt, a.Marks, sequence, a.ID); err != nil {
				return err
			}
			if a.Digest != nil {
				if _, err := tx.ExecContext(ctx,
					`UPDATE answer_validators SET expected_hmac = ? WHERE question_id = ? AND sequence_no = 1`,
					a.Digest, a.ID); err != nil {
					return err
				}
			}
			continue
		}
		if a.Digest == nil {
			return fmt.Errorf("%w: answer %d needs its expected answer", levels.ErrInvalidInput, sequence)
		}
		childCode := ""
		for ; childCode == "" || used[childCode]; next++ {
			childCode = code + "-a" + strconv.Itoa(next)
		}
		used[childCode] = true
		childID, err := insertedID(tx.ExecContext(ctx,
			`INSERT INTO assessment_questions
            (assessment_version_id, parent_question_id, code, prompt, answer_type, points, sequence_no)
            VALUES (?, ?, ?, ?, 'flag', ?, ?)`,
			versionID, questionID, childCode, a.Prompt, a.Marks, sequence))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO answer_validators (question_id, validator_type, expected_hmac)
            VALUES (?, 'case_insensitive_hmac', ?)`,
			childID, a.Digest); err != nil {
			return err
		}
	}
	return nil
}

// labFor returns the published lab template version for image, creating a
// template named after the image when none exists, or nil for no image.
// Questions sharing an image share its template.
func labFor(ctx context.Context, tx *sql.Tx, image string, adminID uint64) (any, error) {
	if image == "" {
		return nil, nil
	}
	var versionID uint64
	err := tx.QueryRowContext(ctx,
		`SELECT v.id FROM lab_templates t
        JOIN lab_template_versions v ON v.lab_template_id = t.id AND v.status = 'published'
        WHERE t.slug = ?`,
		image).Scan(&versionID)
	if err == nil {
		return versionID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	templateID, err := insertedID(tx.ExecContext(ctx,
		`INSERT INTO lab_templates (name, slug, description, status, created_by)
        VALUES (?, ?, ?, 'active', ?)`,
		image, image, "Lab image "+image, adminID))
	if err != nil {
		return nil, err
	}
	spec := "image: " + image + "\n"
	digest := sha256.Sum256([]byte(spec))
	versionID, err = insertedID(tx.ExecContext(ctx,
		`INSERT INTO lab_template_versions
        (lab_template_id, version_no, yaml_spec, spec_sha256, status, published_at, created_by)
        VALUES (?, 1, ?, ?, 'published', UTC_TIMESTAMP(6), ?)`,
		templateID, spec, digest[:], adminID))
	if err != nil {
		return nil, err
	}
	return versionID, nil
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func fileOf(ctx context.Context, q querier, versionID, questionID, fileID uint64) (levels.File, error) {
	var f levels.File
	err := q.QueryRowContext(ctx,
		`SELECT f.id, f.original_name, f.size_bytes, f.stored_name
        FROM question_files f
        JOIN assessment_questions q ON q.id = f.question_id
        WHERE f.id = ? AND f.question_id = ? AND q.assessment_version_id = ?`,
		fileID, questionID, versionID).Scan(&f.ID, &f.Name, &f.Size, &f.StoredName)
	if errors.Is(err, sql.ErrNoRows) {
		return f, levels.ErrNotFound
	}
	return f, err
}

func storedNames(ctx context.Context, tx *sql.Tx, query string, id uint64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// freeSlug derives a slug from name that no level uses yet.
func freeSlug(ctx context.Context, tx *sql.Tx, name string) (string, error) {
	base := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "level"
	}
	if len(base) > 100 {
		base = strings.TrimRight(base[:100], "-")
	}
	slug := base
	for n := 2; ; n++ {
		var taken int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM levels WHERE slug = ?`, slug).Scan(&taken); err != nil {
			return "", err
		}
		if taken == 0 {
			return slug, nil
		}
		slug = base + "-" + strconv.Itoa(n)
	}
}
