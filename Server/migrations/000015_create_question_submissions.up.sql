CREATE TABLE question_submissions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    attempt_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,

    sequence_no SMALLINT UNSIGNED NOT NULL,

    answer_digest BINARY(32) NOT NULL,
    answer_payload JSON NULL,

    is_correct BOOLEAN NULL,
    points_awarded DECIMAL(10,2) UNSIGNED NULL,

    evaluated_at DATETIME(6) NULL,
    evaluated_by BIGINT UNSIGNED NULL,

    submitted_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_question_submissions_sequence
        UNIQUE (
            attempt_id,
            question_id,
            sequence_no
        ),

    CONSTRAINT uq_question_submissions_identity
        UNIQUE (
            id,
            attempt_id,
            question_id
        ),

    CONSTRAINT chk_question_submissions_sequence
        CHECK (sequence_no > 0),

    CONSTRAINT chk_question_submissions_evaluation
        CHECK (
            (
                is_correct IS NULL
                AND points_awarded IS NULL
                AND evaluated_at IS NULL
                AND evaluated_by IS NULL
            )
            OR
            (
                is_correct IS NOT NULL
                AND points_awarded IS NOT NULL
                AND evaluated_at IS NOT NULL
            )
        ),

    CONSTRAINT fk_question_submissions_attempt
        FOREIGN KEY (attempt_id)
        REFERENCES attempts (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT fk_question_submissions_question
        FOREIGN KEY (question_id)
        REFERENCES assessment_questions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_question_submissions_evaluated_by
        FOREIGN KEY (evaluated_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_question_submissions_attempt_question (
        attempt_id,
        question_id,
        submitted_at
    ),

    INDEX idx_question_submissions_pending_evaluation (
        is_correct,
        submitted_at
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;