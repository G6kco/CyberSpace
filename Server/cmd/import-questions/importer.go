package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	"github.com/G6kco/CyberSpace/internal/evaluation"
)

// Level 1 settings decided by the project owner on 2026-10-07.
const (
	levelSequence   = 1
	levelName       = "Level 1"
	levelSlug       = "level-1"
	assessmentSlug  = "level-1-practical"
	assessmentTitle = "Level 1 Practical"
	durationSeconds = 3600
	passScore       = 80
	totalPoints     = 2 * questionPoints // one question from each pool
	instructions    = "You are given one Wireshark question and one Nmap question. " +
		"Download the capture file and analyse it in Wireshark; scan the target IP " +
		"shown for the Nmap question. Submit each flag as you find it. " +
		"You have one hour and need 80 of 100 points to pass."
)

var errAlreadyImported = errors.New("the level 1 assessment already exists; nothing was changed")

type summary struct {
	questions, flags, images int
}

// importBank writes the question bank as Level 1's published assessment in
// one transaction, so a failed import leaves nothing behind.
func importBank(
	ctx context.Context,
	db *sql.DB,
	key []byte,
	adminEmail string,
	bank []question,
) (summary, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return summary{}, err
	}
	defer tx.Rollback()

	var adminID uint64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = ? AND role = 'admin' AND status = 'active'`,
		adminEmail,
	).Scan(&adminID)
	if errors.Is(err, sql.ErrNoRows) {
		return summary{}, fmt.Errorf("no active admin with email %q", adminEmail)
	}
	if err != nil {
		return summary{}, err
	}

	levelID, err := ensureLevel(ctx, tx)
	if err != nil {
		return summary{}, err
	}

	var existing uint64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM assessments WHERE level_id = ? AND slug = ?`,
		levelID, assessmentSlug,
	).Scan(&existing)
	if err == nil {
		return summary{}, errAlreadyImported
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return summary{}, err
	}

	assessmentID, err := insert(ctx, tx,
		`INSERT INTO assessments (level_id, title, slug, created_by) VALUES (?, ?, ?, ?)`,
		levelID, assessmentTitle, assessmentSlug, adminID)
	if err != nil {
		return summary{}, fmt.Errorf("insert assessment: %w", err)
	}
	versionID, err := insert(ctx, tx,
		`INSERT INTO assessment_versions
        (assessment_id, version_no, instructions, duration_seconds, pass_score,
         total_points, max_attempts, status, published_at, created_by)
        VALUES (?, 1, ?, ?, ?, ?, 1, 'published', UTC_TIMESTAMP(6), ?)`,
		assessmentID, instructions, durationSeconds, passScore, totalPoints, adminID)
	if err != nil {
		return summary{}, fmt.Errorf("insert assessment version: %w", err)
	}
	// Status and current version change together to satisfy
	// chk_assessments_current_version.
	if _, err := tx.ExecContext(ctx,
		`UPDATE assessments SET status = 'published', current_version_id = ? WHERE id = ?`,
		versionID, assessmentID,
	); err != nil {
		return summary{}, fmt.Errorf("publish assessment: %w", err)
	}

	var result summary
	labs := map[string]uint64{}
	seq := map[string]int{}
	for i, q := range bank {
		var labID any
		var pcapFile any
		switch q.pool {
		case "nmap":
			id, ok := labs[q.image]
			if !ok {
				if id, err = ensureLab(ctx, tx, q.image, adminID); err != nil {
					return summary{}, err
				}
				labs[q.image] = id
				result.images++
			}
			labID = id
		case "wireshark":
			pcapFile = q.pcapFile
		}

		seq[q.pool]++
		code := fmt.Sprintf("%s-%02d", q.pool, seq[q.pool])
		groupID, err := insert(ctx, tx,
			`INSERT INTO assessment_questions
            (assessment_version_id, code, prompt, answer_type, pool, difficulty,
             lab_template_version_id, pcap_file, points, sequence_no)
            VALUES (?, ?, ?, 'group', ?, ?, ?, ?, 0, ?)`,
			versionID, code, q.prompt, q.pool, q.difficulty, labID, pcapFile, i+1)
		if err != nil {
			return summary{}, fmt.Errorf("insert question %d: %w", q.dumpID, err)
		}

		for j, f := range q.flags {
			answerType, childCode := "flag", code+"-flag"
			if j > 0 {
				answerType, childCode = "text", fmt.Sprintf("%s-q%d", code, j)
			}
			childID, err := insert(ctx, tx,
				`INSERT INTO assessment_questions
                (assessment_version_id, parent_question_id, code, prompt,
                 answer_type, points, sequence_no)
                VALUES (?, ?, ?, ?, ?, ?, ?)`,
				versionID, groupID, childCode, f.prompt, answerType, f.points, j+1)
			if err != nil {
				return summary{}, fmt.Errorf("insert question %d flag %d: %w", q.dumpID, j+1, err)
			}

			digest := evaluation.Digest(key, f.answer)
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO answer_validators (question_id, validator_type, expected_hmac)
                VALUES (?, 'case_insensitive_hmac', ?)`,
				childID, digest[:],
			); err != nil {
				return summary{}, fmt.Errorf("insert question %d validator %d: %w", q.dumpID, j+1, err)
			}
			result.flags++
		}
		result.questions++
	}

	return result, tx.Commit()
}

func ensureLevel(ctx context.Context, tx *sql.Tx) (uint64, error) {
	var id uint64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM levels WHERE sequence_no = ?`, levelSequence,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		id, err = insert(ctx, tx,
			`INSERT INTO levels (name, slug, sequence_no, status) VALUES (?, ?, ?, 'published')`,
			levelName, levelSlug, levelSequence)
	}
	if err != nil {
		return 0, fmt.Errorf("level %d: %w", levelSequence, err)
	}
	return id, nil
}

// ensureLab returns the published lab template version for image, creating
// a template named after the image when none exists.
func ensureLab(ctx context.Context, tx *sql.Tx, image string, adminID uint64) (uint64, error) {
	var versionID uint64
	err := tx.QueryRowContext(ctx,
		`SELECT v.id
        FROM lab_templates t
        JOIN lab_template_versions v
          ON v.lab_template_id = t.id AND v.status = 'published'
        WHERE t.slug = ?`,
		image,
	).Scan(&versionID)
	if err == nil {
		return versionID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	templateID, err := insert(ctx, tx,
		`INSERT INTO lab_templates (name, slug, description, status, created_by)
        VALUES (?, ?, ?, 'active', ?)`,
		image, image, "Nmap target "+image, adminID)
	if err != nil {
		return 0, fmt.Errorf("insert lab template %s: %w", image, err)
	}
	spec := "image: " + image + "\n"
	digest := sha256.Sum256([]byte(spec))
	versionID, err = insert(ctx, tx,
		`INSERT INTO lab_template_versions
        (lab_template_id, version_no, yaml_spec, spec_sha256, status, published_at, created_by)
        VALUES (?, 1, ?, ?, 'published', UTC_TIMESTAMP(6), ?)`,
		templateID, spec, digest[:], adminID)
	if err != nil {
		return 0, fmt.Errorf("insert lab template version %s: %w", image, err)
	}
	return versionID, nil
}

func insert(ctx context.Context, tx *sql.Tx, query string, args ...any) (uint64, error) {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}
