-- The top-level questions dealt to an attempt: for a level assessment, one
-- from each pool. Submissions are accepted only for questions dealt here and
-- their children.
CREATE TABLE attempt_questions (
    attempt_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    sequence_no SMALLINT UNSIGNED NOT NULL,
    assigned_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (
        attempt_id,
        question_id
    ),
    CONSTRAINT uq_attempt_questions_sequence
        UNIQUE (attempt_id, sequence_no),
    CONSTRAINT chk_attempt_questions_sequence
        CHECK (sequence_no > 0),
    CONSTRAINT fk_attempt_questions_attempt
        FOREIGN KEY (attempt_id)
        REFERENCES attempts (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,
    CONSTRAINT fk_attempt_questions_question
        FOREIGN KEY (question_id)
        REFERENCES assessment_questions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    INDEX idx_attempt_questions_question (
        question_id
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;
